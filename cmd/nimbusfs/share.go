package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"nimbusfs/internal/config"
	"nimbusfs/internal/fsops"
	"nimbusfs/internal/store"
)

func cmdShare(args []string) {
	fs := flag.NewFlagSet("share", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "Path to config file")
	mode := fs.String("mode", "both", "Access mode: both, view_only, or download_only")
	password := fs.String("password", "", "Require this password to access the share")
	expires := fs.String("expires", "", "Expiry, e.g. 30m, 24h, 7d (default: never)")
	baseURL := fs.String("base-url", "", "Server base URL, used only to print the full link")
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: nimbusfs share [flags] <path>")
		os.Exit(1)
	}
	inputPath := fs.Arg(0)

	switch *mode {
	case "both", "view_only", "download_only":
	default:
		fmt.Fprintln(os.Stderr, "error: -mode must be one of: both, view_only, download_only")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading config: %v\n", err)
		os.Exit(1)
	}
	sandbox, err := fsops.NewSandbox(cfg.Filesystem.Root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: setting up sandbox: %v\n", err)
		os.Exit(1)
	}

	relPath, err := resolveSharePath(sandbox, inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var expiresAt time.Time
	if *expires != "" {
		d, err := parseExpiry(*expires)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid -expires: %v\n", err)
			os.Exit(1)
		}
		expiresAt = time.Now().Add(d)
	}

	var passwordHash string
	if *password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: hashing password: %v\n", err)
			os.Exit(1)
		}
		passwordHash = string(hash)
	}

	currentUser, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: looking up current user: %v\n", err)
		os.Exit(1)
	}

	token, err := generateToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: generating token: %v\n", err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening data store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	share := store.Share{
		Token:        token,
		Username:     currentUser.Username,
		Path:         relPath,
		Mode:         *mode,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		ExpiresAt:    expiresAt,
	}
	if err := st.CreateShare(share); err != nil {
		if strings.Contains(err.Error(), "readonly database") {
			fmt.Fprintln(os.Stderr, "error: cannot write to the nimbusfs database — it's owned by whichever "+
				"user runs 'nimbusfs serve' (typically root). Run this command with the same privilege, e.g. via sudo.")
		} else {
			fmt.Fprintf(os.Stderr, "error: creating share: %v\n", err)
		}
		os.Exit(1)
	}

	link := "/share/" + token
	if *baseURL != "" {
		link = strings.TrimRight(*baseURL, "/") + link
	}
	fmt.Println(link)
}

// resolveSharePath validates that inputPath (an absolute filesystem path)
// is within the configured root and actually accessible to the invoking
// user — this command runs as whoever calls it, with no impersonation, so
// a normal stat/permission check is exactly the access control we want.
func resolveSharePath(sandbox *fsops.Sandbox, inputPath string) (string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return "", err
	}
	guess, err := sandbox.RelPath(absInput)
	if err != nil {
		return "", fmt.Errorf("%s is not within the configured filesystem.root", inputPath)
	}
	canonical, err := sandbox.Resolve(guess)
	if err != nil {
		return "", fmt.Errorf("%s is not a valid path under filesystem.root: %w", inputPath, err)
	}
	if _, err := os.Stat(canonical); err != nil {
		return "", fmt.Errorf("cannot access %s: %w", inputPath, err)
	}
	return sandbox.RelPath(canonical)
}

func parseExpiry(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
