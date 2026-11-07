package main

import (
	"context"
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
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInit(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	panelDomain := flags.String("panel-domain", "", "")
	hy2Domain := flags.String("hy2-domain", "", "")
	publicHost := flags.String("public-host", "", "")
	adminIP := flags.String("admin-ip", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *panelDomain == "" || *publicHost == "" || *adminIP == "" {
		return errors.New("init 需要 panel-domain、public-host 和 admin-ip")
	}
	if _, err := os.Stat(*path); err == nil {
		return errors.New("配置已存在，拒绝覆盖")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	passwordBytes, err := io.ReadAll(io.LimitReader(os.Stdin, 1025))
	if err != nil {
		return err
	}
	password := strings.TrimRight(string(passwordBytes), "\r\n")
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
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	panelStore := store.New(*path)
	applier := server.FileApplier{Dir: filepath.Dir(*path)}
	return server.New(panelStore, applier).Serve(ctx)
}

func runRender(args []string) error {
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	output := flags.String("output", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *output == "" {
		return errors.New("render 需要 output")
	}
	cfg, err := store.New(*path).Load()
	if err != nil {
		return err
	}
	data, err := cfg.SingBoxJSON()
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(*output)
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
	path := flags.String("config", "/etc/hy2panel/panel.json", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := store.New(*path).Load()
	if err != nil {
		return err
	}
	if _, err := cfg.SingBoxJSON(); err != nil {
		return err
	}
	fmt.Println("配置有效")
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法: hy2panel <init|serve|check|render> [参数]")
}
