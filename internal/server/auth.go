package server

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionLifetime = 8 * time.Hour

type session struct {
	expires time.Time
}

type authenticator struct {
	mu            sync.Mutex
	sessions      map[string]session
	failedLogins  int
	loginLockedTo time.Time
}

func newAuthenticator() *authenticator {
	return &authenticator{sessions: make(map[string]session)}
}

func (a *authenticator) login(password, hash string) (string, error) {
	a.mu.Lock()
	if time.Now().Before(a.loginLockedTo) {
		a.mu.Unlock()
		return "", errors.New("登录失败次数过多，请稍后重试")
	}
	a.mu.Unlock()
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		a.mu.Lock()
		a.failedLogins++
		if a.failedLogins >= 5 {
			a.loginLockedTo = time.Now().Add(5 * time.Minute)
			a.failedLogins = 0
		}
		a.mu.Unlock()
		return "", errors.New("密码错误")
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	a.mu.Lock()
	now := time.Now()
	for key, item := range a.sessions {
		if now.After(item.expires) {
			delete(a.sessions, key)
		}
	}
	a.sessions[token] = session{expires: now.Add(sessionLifetime)}
	a.failedLogins = 0
	a.loginLockedTo = time.Time{}
	a.mu.Unlock()
	return token, nil
}

func (a *authenticator) valid(token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	item, ok := a.sessions[token]
	if !ok || time.Now().After(item.expires) {
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
