package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Listen      string `yaml:"listen"`
	BehindProxy bool   `yaml:"behind_proxy"`
}

type FilesystemConfig struct {
	Root string `yaml:"root"`
}

type AuthConfig struct {
	PAM       bool `yaml:"pam"`
	SSHKeys   bool `yaml:"ssh_keys"`
	ProxyAuth bool `yaml:"proxy_auth"`
}

type SharingConfig struct {
	Enabled bool `yaml:"enabled"`
}

type SearchConfig struct {
	Enabled bool `yaml:"enabled"`
}

type UIConfig struct {
	DarkMode bool `yaml:"dark_mode"`
}

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Filesystem FilesystemConfig `yaml:"filesystem"`
	Auth       AuthConfig       `yaml:"auth"`
	Sharing    SharingConfig    `yaml:"sharing"`
	Search     SearchConfig     `yaml:"search"`
	UI         UIConfig         `yaml:"ui"`

	// DataDir holds sqlite db, sessions, thumbnail cache. Derived, not from yaml.
	DataDir string `yaml:"data_dir"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:      "127.0.0.1:8080",
			BehindProxy: false,
		},
		Filesystem: FilesystemConfig{
			Root: "/srv/files",
		},
		Auth: AuthConfig{
			PAM:       true,
			SSHKeys:   false,
			ProxyAuth: false,
		},
		Sharing: SharingConfig{Enabled: true},
		Search:  SearchConfig{Enabled: true},
		UI:      UIConfig{DarkMode: true},
		DataDir: "/var/lib/nimbusfs",
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Listen == "" {
		return fmt.Errorf("server.listen must not be empty")
	}
	if c.Filesystem.Root == "" {
		return fmt.Errorf("filesystem.root must not be empty")
	}
	info, err := os.Stat(c.Filesystem.Root)
	if err != nil {
		return fmt.Errorf("filesystem.root %q: %w", c.Filesystem.Root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("filesystem.root %q is not a directory", c.Filesystem.Root)
	}
	if !c.Auth.PAM && !c.Auth.SSHKeys && !c.Auth.ProxyAuth {
		return fmt.Errorf("at least one auth method must be enabled")
	}
	if c.DataDir == "" {
		return fmt.Errorf("data_dir must not be empty")
	}
	return nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
