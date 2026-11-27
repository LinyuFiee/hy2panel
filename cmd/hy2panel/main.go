package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"hy2panel/internal/config"
	"hy2panel/internal/server"
	"hy2panel/internal/store"
	"hy2panel/internal/version"
)

var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "check":
		err = runCheck(os.Args[2:])
	case "render":
		err = runRender(os.Args[2:])
	case "version":
		err = runVersion(os.Args[2:])
	case "migrate":
		err = runMigrate(os.Args[2:])
	case "reset-password":
		err = runResetPassword(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func runInit(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	panelDomain := flags.String("panel-domain", "", "")
	hy2Domain := flags.String("hy2-domain", "", "")
	publicHost := flags.String("public-host", "", "")
	adminIP := flags.String("admin-ip", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("init 不接受位置参数")
	}
	if *panelDomain == "" || *publicHost == "" {
		return errors.New("init 需要 panel-domain 和 public-host")
	}
	if _, err := os.Stat(*path); err == nil {
		return errors.New("配置已存在，拒绝覆盖")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	password, err := readPassword(stdin)
	if err != nil {
		return err
	}
	cfg, err := config.Initial(*panelDomain, *hy2Domain, *publicHost, *adminIP, password)
	if err != nil {
		return err
	}
	cfg.RackNerd.APIKey = os.Getenv("RACKNERD_API_KEY")
	cfg.RackNerd.APIHash = os.Getenv("RACKNERD_API_HASH")
	return store.New(*path).Save(cfg)
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("serve 不接受位置参数")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	panelStore := store.New(*path)
	applier := server.FileApplier{Dir: filepath.Dir(*path)}
	return server.New(panelStore, applier).Serve(ctx)
}

func runRender(args []string) error {
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	output := flags.String("output", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("render 不接受位置参数")
	}
	if *output == "" {
		return errors.New("render 需要 output")
	}
	cfg, setOwner, err := loadRenderConfig(*path, *output)
	if err != nil {
		return err
	}
	data, err := cfg.SingBoxJSON()
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(*output)
	if setOwner != nil {
		// Once chowned, the panel owns the inode. Keep its temporary name
		// private until the fully prepared file is renamed into place.
		dir, err = os.MkdirTemp(dir, ".render-*")
		if err != nil {
			return err
		}
		defer os.Remove(dir)
	}
	temp, err := os.CreateTemp(dir, ".sing-box-*.tmp")
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
	if setOwner != nil {
		if err := setOwner(temp); err != nil {
			temp.Close()
			return err
		}
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, *output)
}

func runCheck(args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("check 不接受位置参数")
	}
	cfg, err := store.New(*path).Load()
	if err != nil {
		return err
	}
	if _, err := cfg.SingBoxJSON(); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "配置有效")
	return nil
}

func runVersion(args []string) error {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("version 不接受位置参数")
	}
	if *asJSON {
		return json.NewEncoder(stdout).Encode(map[string]any{
			"version":        version.Version,
			"config_version": config.SchemaVersion,
		})
	}
	fmt.Fprintln(stdout, version.Version)
	return nil
}

func runMigrate(args []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("migrate 不接受位置参数")
	}
	changed, err := store.New(*path).Migrate()
	if err != nil {
		return err
	}
	if changed {
		fmt.Fprintln(stdout, "配置迁移完成")
	} else {
		fmt.Fprintln(stdout, "配置无需迁移")
	}
	return nil
}

func runResetPassword(args []string) error {
	flags := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("reset-password 不接受位置参数")
	}
	password, err := readPassword(stdin)
	if err != nil {
		return err
	}
	hash, err := config.HashAdminPassword(password)
	if err != nil {
		return err
	}
	_, err = store.New(*path).Update(func(cfg *config.Config) error {
		cfg.AdminPasswordHash = hash
		return nil
	})
	return err
}

func readPassword(reader io.Reader) (string, error) {
	passwordBytes, err := io.ReadAll(io.LimitReader(reader, 1025))
	if err != nil {
		return "", err
	}
	if len(passwordBytes) > 1024 {
		return "", errors.New("密码输入过长")
	}
	return strings.TrimRight(string(passwordBytes), "\r\n"), nil
}

func usage() {
	fmt.Fprintln(stderr, "用法: hy2panel <init|serve|check|render|version|migrate|reset-password> [参数]")
}
