//go:build !linux

package main

import (
	"os"

	"hy2panel/internal/config"
	"hy2panel/internal/store"
)

func loadRenderConfig(path, output string) (*config.Config, func(*os.File) error, error) {
	cfg, err := store.New(path).Load()
	return cfg, nil, err
}
