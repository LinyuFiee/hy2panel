package server

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionLifetime       = 8 * time.Hour
	loginWindow           = 5 * time.Minute
	maxLoginFailures      = 5
	maxLoginSources       = 4096
	maxConcurrentBcrypt   = 4
	unknownLoginSourceKey = "unknown"
)

var (
	errInvalidPassword = errors.New("密码错误")
	errLoginLimited    = errors.New("登录失败次数过多，请稍后重试")
	errLoginBusy       = errors.New("登录验证正在处理中，请稍后重试")
)

type session struct {
	expires time.Time
}

type loginFailures struct {
	failures int
	resetAt  time.Time
}

type authenticator struct {
	mu            sync.Mutex
	sessions      map[string]session
	failedLogins  map[string]loginFailures
	pendingLogins map[string]int
	inflightTotal int
	now           func() time.Time
	bcryptSlots   chan struct{}
}

func newAuthenticator() *authenticator {
	return newAuthenticatorWithClock(time.Now)
}

func newAuthenticatorWithClock(now func() time.Time) *authenticator {
	if now == nil {
		now = time.Now
	}
	return &authenticator{
		sessions:      make(map[string]session),
		failedLogins:  make(map[string]loginFailures),
		pendingLogins: make(map[string]int),
		now:           now,
		bcryptSlots:   make(chan struct{}, maxConcurrentBcrypt),
	}
}

func (a *authenticator) login(source, password, hash string) (string, time.Duration, error) {
	key := normalizeSourceIP(source)
	now := a.now()
	a.mu.Lock()
	a.cleanupLocked(now)
	failures := a.failedLogins[key]
	if failures.failures >= maxLoginFailures && now.Before(failures.resetAt) {
		retryAfter := failures.resetAt.Sub(now)
		a.mu.Unlock()
		return "", retryAfter, errLoginLimited
	}
	if failures.failures+a.pendingLogins[key] >= maxLoginFailures || a.inflightTotal >= maxConcurrentBcrypt {
		a.mu.Unlock()
		return "", 0, errLoginBusy
	}
	a.pendingLogins[key]++
	a.inflightTotal++
	a.mu.Unlock()
	a.bcryptSlots <- struct{}{}
	defer func() { <-a.bcryptSlots }()
	return a.finishLogin(key, password, hash)
}

func (a *authenticator) finishLogin(key, password, hash string) (string, time.Duration, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	now := a.now()
	a.mu.Lock()
	a.pendingLogins[key]--
	a.inflightTotal--
	if a.pendingLogins[key] == 0 {
		delete(a.pendingLogins, key)
	}
	a.cleanupLocked(now)
	failures := a.failedLogins[key]
	if err != nil {
		if failures.resetAt.IsZero() || !now.Before(failures.resetAt) {
			failures = loginFailures{resetAt: now.Add(loginWindow)}
		}
		failures.failures++
		if failures.failures > maxLoginFailures {
			failures.failures = maxLoginFailures
		}
		a.failedLogins[key] = failures
		a.enforceSourceCapacityLocked()
		retryAfter := time.Duration(0)
		if failures.failures >= maxLoginFailures {
			retryAfter = failures.resetAt.Sub(now)
		}
		a.mu.Unlock()
		return "", retryAfter, errInvalidPassword
	}

	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		a.mu.Unlock()
		return "", 0, err
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	a.sessions[token] = session{expires: now.Add(sessionLifetime)}
	delete(a.failedLogins, key)
	a.mu.Unlock()
	return token, 0, nil
}

func (a *authenticator) valid(token string) bool {
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	item, ok := a.sessions[token]
	if !ok || !now.Before(item.expires) {
		delete(a.sessions, token)
		return false
	}
	return true
}

func (a *authenticator) logout(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

func (a *authenticator) cleanupLocked(now time.Time) {
	for key, item := range a.sessions {
		if !now.Before(item.expires) {
			delete(a.sessions, key)
		}
	}
	for key, item := range a.failedLogins {
		if !now.Before(item.resetAt) {
			delete(a.failedLogins, key)
		}
	}
}

func (a *authenticator) enforceSourceCapacityLocked() {
	for len(a.failedLogins) > maxLoginSources {
		var oldestKey string
		var oldest time.Time
		for key, item := range a.failedLogins {
			if oldestKey == "" || item.resetAt.Before(oldest) {
				oldestKey = key
				oldest = item.resetAt
			}
		}
		delete(a.failedLogins, oldestKey)
	}
}

func normalizeSourceIP(source string) string {
	addr, err := netip.ParseAddr(source)
	if err != nil {
		return unknownLoginSourceKey
	}
	return addr.Unmap().String()
}

func sessionCookie(token string, expires time.Time, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "__Host-hy2panel",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   maxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}
