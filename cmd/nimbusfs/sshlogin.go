package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

func cmdSSHLogin(args []string) {
	fs := flag.NewFlagSet("ssh-login", flag.ExitOnError)
	server := fs.String("server", "", "Base URL of the nimbusfs server (e.g. https://files.example.com)")
	code := fs.String("code", "", "Device code shown in the browser")
	keyPath := fs.String("key", "", "Path to a private key file (defaults to using ssh-agent)")
	_ = fs.Parse(args)

	if *server == "" {
		fmt.Fprintln(os.Stderr, "error: -server is required")
		os.Exit(1)
	}
	base := strings.TrimRight(*server, "/")

	*code = strings.ToUpper(strings.TrimSpace(*code))
	if *code == "" {
		fmt.Print("Enter the code shown in your browser: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		*code = strings.ToUpper(strings.TrimSpace(line))
	}
	if *code == "" {
		fmt.Fprintln(os.Stderr, "error: a device code is required")
		os.Exit(1)
	}

	device, err := fetchDeviceInfo(base, *code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	signers, err := loadSigners(*keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(signers) == 0 {
		fmt.Fprintln(os.Stderr, "error: no SSH keys available (start ssh-agent and ssh-add a key, or pass -key)")
		os.Exit(1)
	}

	signer := pickSigner(signers, device.Fingerprints)
	if signer == nil {
		fmt.Fprintf(os.Stderr, "error: none of your local keys are authorized for %s; available local keys don't match the account's authorized_keys\n", device.Username)
		os.Exit(1)
	}

	nonce, err := base64.StdEncoding.DecodeString(device.Nonce)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: server returned an invalid challenge")
		os.Exit(1)
	}

	sig, err := signer.Sign(rand.Reader, nonce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: signing failed: %v\n", err)
		os.Exit(1)
	}

	if err := submitSignature(base, *code, signer.PublicKey(), sig); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Signed in as %s. Return to your browser to continue.\n", device.Username)
}

type deviceInfo struct {
	Username     string   `json:"username"`
	Nonce        string   `json:"nonce"`
	Fingerprints []string `json:"fingerprints"`
}

func fetchDeviceInfo(base, code string) (*deviceInfo, error) {
	resp, err := http.Get(base + "/api/auth/ssh/device?code=" + code)
	if err != nil {
		return nil, fmt.Errorf("contacting server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
	}
	var info deviceInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding server response: %w", err)
	}
	return &info, nil
}

func submitSignature(base, code string, pub ssh.PublicKey, sig *ssh.Signature) error {
	body, err := json.Marshal(map[string]string{
		"code":      code,
		"publicKey": base64.StdEncoding.EncodeToString(pub.Marshal()),
		"sigFormat": sig.Format,
		"signature": base64.StdEncoding.EncodeToString(sig.Blob),
	})
	if err != nil {
		return err
	}
	resp, err := http.Post(base+"/api/auth/ssh/respond", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("contacting server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeAPIError(resp)
	}
	return nil
}

func decodeAPIError(resp *http.Response) error {
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && body.Error != "" {
		return fmt.Errorf("%s", body.Error)
	}
	return fmt.Errorf("server returned %s", resp.Status)
}

// loadSigners returns available signers either from an explicit key file
// or, by default, from a running ssh-agent (SSH_AUTH_SOCK).
func loadSigners(keyPath string) ([]ssh.Signer, error) {
	if keyPath != "" {
		signer, err := loadSignerFromFile(keyPath)
		if err != nil {
			return nil, err
		}
		return []ssh.Signer{signer}, nil
	}

	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK is not set; start ssh-agent or pass -key")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("connecting to ssh-agent: %w", err)
	}
	client := agent.NewClient(conn)
	return client.Signers()
}

func loadSignerFromFile(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		return signer, nil
	}
	var passErr *ssh.PassphraseMissingError
	if !errors.As(err, &passErr) {
		return nil, fmt.Errorf("parsing key file: %w", err)
	}
	fmt.Print("Enter passphrase for key: ")
	passphrase, readErr := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if readErr != nil {
		return nil, fmt.Errorf("reading passphrase: %w", readErr)
	}
	signer, err = ssh.ParsePrivateKeyWithPassphrase(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("parsing key file: %w", err)
	}
	return signer, nil
}

// pickSigner returns the first available signer whose SHA256 fingerprint is
// in the server-advertised list of acceptable keys for this account.
func pickSigner(signers []ssh.Signer, fingerprints []string) ssh.Signer {
	accepted := make(map[string]bool, len(fingerprints))
	for _, fp := range fingerprints {
		accepted[fp] = true
	}
	for _, s := range signers {
		if accepted[ssh.FingerprintSHA256(s.PublicKey())] {
			return s
		}
	}
	return nil
}
