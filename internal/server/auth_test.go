package server

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func testPasswordHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password-123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

func TestAuthenticatorLimitsFailuresPerIP(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	auth := newAuthenticatorWithClock(func() time.Time { return now })
	hash := testPasswordHash(t)
	for i := 0; i < maxLoginFailures; i++ {
		_, retryAfter, err := auth.login("198.51.100.1", "wrong-password", hash)
		if err != errInvalidPassword {
			t.Fatalf("failure %d = %v, %v", i+1, retryAfter, err)
		}
		if i < maxLoginFailures-1 && retryAfter != 0 {
			t.Fatalf("early failure %d returned retry %v", i+1, retryAfter)
		}
		if i == maxLoginFailures-1 && retryAfter != loginWindow {
			t.Fatalf("threshold failure returned retry %v", retryAfter)
		}
	}
	if _, retryAfter, err := auth.login("198.51.100.1", "test-password-123456", hash); err != errLoginLimited || retryAfter != loginWindow {
		t.Fatalf("expected lockout, got retry=%v err=%v", retryAfter, err)
	}
	if _, _, err := auth.login("198.51.100.2", "test-password-123456", hash); err != nil {
		t.Fatalf("different source was limited: %v", err)
	}
	now = now.Add(loginWindow)
	if _, _, err := auth.login("198.51.100.1", "test-password-123456", hash); err != nil {
		t.Fatalf("expired source limit remained active: %v", err)
	}
}

func TestAuthenticatorNormalizesMappedIPv4(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	auth := newAuthenticatorWithClock(func() time.Time { return now })
	hash := testPasswordHash(t)
	for i := 0; i < maxLoginFailures; i++ {
		if _, _, err := auth.login("::ffff:198.51.100.9", "wrong-password", hash); err != errInvalidPassword {
			t.Fatal(err)
		}
	}
	if _, _, err := auth.login("198.51.100.9", "test-password-123456", hash); err != errLoginLimited {
		t.Fatalf("mapped IPv4 did not share limiter state: %v", err)
	}
}

func TestAuthenticatorConcurrentAttemptsCannotBypassPerIPLimit(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	auth := newAuthenticatorWithClock(func() time.Time { return now })
	hash := testPasswordHash(t)
	const attempts = 24
	start := make(chan struct{})
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := auth.login("198.51.100.88", "wrong-password", hash)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	invalid, rejected := 0, 0
	for err := range results {
		switch err {
		case errInvalidPassword:
			invalid++
		case errLoginBusy, errLoginLimited:
			rejected++
		default:
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if invalid > maxLoginFailures || invalid+rejected != attempts || invalid == 0 {
		t.Fatalf("bcrypt attempts=%d rejected=%d", invalid, rejected)
	}
}

func TestAuthenticatorSuccessfulLoginClearsFailures(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	auth := newAuthenticatorWithClock(func() time.Time { return now })
	hash := testPasswordHash(t)
	for i := 0; i < maxLoginFailures-1; i++ {
		if _, _, err := auth.login("198.51.100.3", "wrong-password", hash); err != errInvalidPassword {
			t.Fatal(err)
		}
	}
	token, _, err := auth.login("198.51.100.3", "test-password-123456", hash)
	if err != nil || token == "" || !auth.valid(token) {
		t.Fatalf("successful login failed: token=%q err=%v", token, err)
	}
	for i := 0; i < maxLoginFailures-1; i++ {
		if _, _, err := auth.login("198.51.100.3", "wrong-password", hash); err != errInvalidPassword {
			t.Fatal(err)
		}
	}
	if _, _, err := auth.login("198.51.100.3", "test-password-123456", hash); err != nil {
		t.Fatalf("failures were not cleared by successful login: %v", err)
	}
	now = now.Add(sessionLifetime)
	if auth.valid(token) {
		t.Fatal("session did not expire according to injected clock")
	}
}

func TestAuthenticatorBoundsAndCleansFailureSources(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	auth := newAuthenticatorWithClock(func() time.Time { return now })
	hash := testPasswordHash(t)
	for i := 0; i < maxLoginSources+16; i++ {
		source := fmt.Sprintf("2001:db8::%x", i+1)
		if _, _, err := auth.login(source, "wrong-password", hash); err != errInvalidPassword {
			t.Fatal(err)
		}
	}
	auth.mu.Lock()
	if got := len(auth.failedLogins); got != maxLoginSources {
		auth.mu.Unlock()
		t.Fatalf("failure source capacity = %d", got)
	}
	auth.mu.Unlock()
	now = now.Add(loginWindow)
	if _, _, err := auth.login("198.51.100.50", "wrong-password", hash); err != errInvalidPassword {
		t.Fatal(err)
	}
	auth.mu.Lock()
	defer auth.mu.Unlock()
	if got := len(auth.failedLogins); got != 1 {
		t.Fatalf("expired failure sources were not cleaned: %d", got)
	}
}

func TestNormalizeSourceIP(t *testing.T) {
	if got := normalizeSourceIP("::ffff:192.0.2.1"); got != "192.0.2.1" {
		t.Fatalf("mapped IPv4 normalized to %q", got)
	}
	if got := normalizeSourceIP("not-an-ip"); got != unknownLoginSourceKey {
		t.Fatalf("invalid source normalized to %q", got)
	}
}
