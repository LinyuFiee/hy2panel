package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"syscall"

	"hy2panel/internal/config"
	"hy2panel/internal/store"
)

const managedDir = "/etc/hy2panel"

func loadRenderConfig(path, output string) (*config.Config, func(*os.File) error, error) {
	if os.Geteuid() != 0 || output != managedDir+"/sing-box.pending.json" {
		cfg, err := store.New(path).Load()
		return cfg, nil, err
	}
	if path != managedDir+"/panel.json" && path != managedDir+"/panel.pending.json" {
		return nil, nil, errors.New("managed pending output requires a managed panel config")
	}
	account, err := user.Lookup("hy2panel")
	if err != nil {
		return nil, nil, err
	}
	group, err := user.LookupGroup("hy2panel")
	if err != nil {
		return nil, nil, err
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil || uid <= 0 {
		return nil, nil, errors.New("invalid hy2panel uid")
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil || gid <= 0 {
		return nil, nil, errors.New("invalid hy2panel gid")
	}
	dir, err := os.Lstat(managedDir)
	if err != nil {
		return nil, nil, err
	}
	stat := dir.Sys().(*syscall.Stat_t)
	if dir.Mode() != os.ModeDir|os.ModeSticky|0o770 || stat.Uid != 0 || int(stat.Gid) != gid {
		return nil, nil, fmt.Errorf("unsafe directory: %s", managedDir)
	}
	cfg, err := readManagedRenderConfig(path, uid, gid)
	return cfg, func(f *os.File) error { return f.Chown(uid, gid) }, err
}

func readManagedRenderConfig(path string, uid, gid int) (*config.Config, error) {
	// The panel can replace this entry in the sticky directory. Check and read
	// the same inode, without following links or blocking on a planted FIFO.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if info.Mode() != 0o600 || stat.Nlink != 1 || int(stat.Uid) != uid || int(stat.Gid) != gid {
		return nil, fmt.Errorf("unsafe managed render input: %s", path)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var cfg config.Config
	err = json.Unmarshal(data, &cfg) // SingBoxJSON validates before rendering.
	return &cfg, err
}
