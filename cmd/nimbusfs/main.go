package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"nimbusfs/internal/auth"
	"nimbusfs/internal/config"
	"nimbusfs/internal/fsops"
	"nimbusfs/internal/search"
	"nimbusfs/internal/server"
	"nimbusfs/internal/store"
	"nimbusfs/internal/thumbnail"
	"nimbusfs/web"
)

const defaultConfigPath = "/etc/nimbusfs/config.yaml"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "init":
		cmdInit(os.Args[2:])
	case "check-config":
		cmdCheckConfig(os.Args[2:])
	case "ssh-login":
		cmdSSHLogin(os.Args[2:])
	case "share":
		cmdShare(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `nimbusfs - self-contained Linux file browser

Usage:
  nimbusfs serve [-config path]         Start the server
  nimbusfs init [-config path]          Write a default config file
  nimbusfs check-config [-config path]  Validate a config file
  nimbusfs ssh-login -server url        Complete a browser's SSH-key login using a local key/ssh-agent
  nimbusfs share [flags] <path>         Create a share link for a file or directory
`)
}

func configPathFlag(args []string) string {
	for i, a := range args {
		if a == "-config" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return defaultConfigPath
}

func cmdInit(args []string) {
	path := configPathFlag(args)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "config already exists at %s\n", path)
		os.Exit(1)
	}
	if err := os.MkdirAll(parentDir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "creating config directory: %v\n", err)
		os.Exit(1)
	}
	cfg := config.Default()
	if err := cfg.Save(path); err != nil {
		fmt.Fprintf(os.Stderr, "writing config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote default config to %s\n", path)
}

func cmdCheckConfig(args []string) {
	path := configPathFlag(args)
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "config invalid: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("config OK")
}

func cmdServe(args []string) {
	path := configPathFlag(args)
	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	if cfg.Auth.PAM && os.Geteuid() != 0 {
		log.Fatal("PAM auth requires nimbusfs to start as root so it can impersonate authenticated Linux users for filesystem access; run with sudo or as a root systemd service")
	}

	sandbox, err := fsops.NewSandbox(cfg.Filesystem.Root)
	if err != nil {
		log.Fatalf("setting up sandbox: %v", err)
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("opening data store: %v", err)
	}
	defer st.Close()

	sessions := auth.NewSessionManager(st)

	searchIndex := search.New(sandbox)
	if cfg.Search.Enabled {
		searchIndex.StartPeriodicRebuild(5 * time.Minute)
	}

	thumbnails := thumbnail.New(filepath.Join(cfg.DataDir, "thumbs"))

	handler := server.New(cfg, sandbox, sessions, st, searchIndex, thumbnails, web.Dist())

	log.Printf("nimbusfs listening on %s (root=%s)", cfg.Server.Listen, sandbox.Root())
	if err := http.ListenAndServe(cfg.Server.Listen, handler); err != nil {
		log.Fatal(err)
	}
}

func parentDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
