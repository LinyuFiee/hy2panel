package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	subscriptionStatePath = "/var/lib/hy2panel/subscription.json"
	subscriptionTokenSize = 32
)

var (
	errSubscriptionDisabled = errors.New("subscription disabled")
	errSubscriptionState    = errors.New("invalid subscription state")
)

// The panel is the single writer. Absence means revoked; a malformed or
// inaccessible file is never treated as absence and never replaced implicitly.
type subscriptionStore struct {
	mu   sync.RWMutex
	path string
}

type subscriptionFile struct {
	Token string `json:"token"`
}

func validSubscriptionToken(token string) bool {
	if len(token) != base64.RawURLEncoding.EncodedLen(subscriptionTokenSize) {
		return false
	}
	bytes, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil && len(bytes) == subscriptionTokenSize
}

func (s *subscriptionStore) current() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.load()
}

func (s *subscriptionStore) change(action string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.load()
	if err != nil {
		return "", err
	}
	switch action {
	case "copy":
		if token == "" {
			return "", errSubscriptionDisabled
		}
		return token, nil
	case "revoke":
		if token == "" {
			return "", nil
		}
		if err := os.Remove(s.path); err != nil {
			return "", err
		}
		return "", syncSubscriptionDir(filepath.Dir(s.path))
	case "create":
		if token != "" {
			return token, nil
		}
	case "rotate":
	default:
		return "", errors.New("invalid subscription action")
	}
	var raw [subscriptionTokenSize]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	next := base64.RawURLEncoding.EncodeToString(raw[:])
	if err := s.save(next); err != nil {
		return "", err
	}
	return next, nil
}

func (s *subscriptionStore) load() (string, error) {
	dir, err := os.Lstat(filepath.Dir(s.path))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !dir.IsDir() || (runtime.GOOS != "windows" && dir.Mode().Perm() != 0o700) {
		return "", errSubscriptionState
	}
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	const maxSize = 1024
	if !info.Mode().IsRegular() || info.Size() > maxSize || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		return "", errSubscriptionState
	}
	file, err := os.Open(s.path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var state subscriptionFile
	decoder := json.NewDecoder(io.LimitReader(file, maxSize+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&state) != nil || !validSubscriptionToken(state.Token) || decoder.Decode(&struct{}{}) != io.EOF {
		return "", errSubscriptionState
	}
	return state.Token, nil
}

func (s *subscriptionStore) save(token string) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// CreateTemp uses 0600. Write and sync in the same directory so a failed
	// pre-rename write never destroys or partially updates the previous token.
	file, err := os.CreateTemp(dir, ".subscription-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := json.NewEncoder(file).Encode(subscriptionFile{Token: token}); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), s.path); err != nil {
		return err
	}
	return syncSubscriptionDir(dir)
}

func syncSubscriptionDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil // Directory fsync is not supported on Windows.
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
