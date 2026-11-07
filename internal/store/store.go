package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"hy2panel/internal/config"
)

type Store struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked()
}

func (s *Store) Save(value *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked(value)
}

func (s *Store) Update(update func(*config.Config) error) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}
	next := *current
	if err := update(&next); err != nil {
		return nil, err
	}
	if err := s.saveUnlocked(&next); err != nil {
		return nil, err
	}
	return &next, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) loadUnlocked() (*config.Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var value config.Config
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("解析面板配置失败: %w", err)
	}
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *Store) saveUnlocked(value *config.Config) error {
	if value == nil {
		return errors.New("配置不能为空")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".panel-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return os.Rename(tempPath, s.path)
}
