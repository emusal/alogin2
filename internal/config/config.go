package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for alogin.
type Config struct {
	// Paths
	DataDir   string // ~/.local/share/alogin  (XDG_DATA_HOME)
	ConfigDir string // ~/.config/alogin        (XDG_CONFIG_HOME)
	DBPath    string // DataDir/alogin.db
	VaultPath string // DataDir/vault.age
	LogFile   string // DataDir/alogin.log

	// Behaviour
	LogLevel         int    // 0=errors, 1=info, 2=debug
	Lang             string // Default locale (ALOGIN_LANG)
	SSHOpt           string // Extra SSH options (ALOGIN_SSHOPT)
	SSHCmd           string // Custom SSH binary (ALOGIN_SSHCMD)
	KeychainUse      bool   // Use OS keychain (ALOGIN_KEYCHAIN_USE)
	PreferredHost    string // Default host for connect with no args
	DefaultGW        string // Default gateway
	DefaultTermTheme string // Fallback terminal theme

	// Legacy compatibility: original ALOGIN_ROOT layout
	LegacyRoot string // ALOGIN_ROOT from environment
}

// Load reads config from environment variables and optional config file.
// Environment variables override config file values.
func Load() (*Config, error) {
	v := viper.New()

	// XDG base directories
	dataHome := xdgDataHome()
	configHome := xdgConfigHome()

	v.SetDefault("data_dir", filepath.Join(dataHome, "alogin"))
	v.SetDefault("config_dir", filepath.Join(configHome, "alogin"))
	v.SetDefault("log_level", 0)
	v.SetDefault("lang", "ko_KR.eucKR")
	v.SetDefault("ssh_cmd", "ssh")
	v.SetDefault("keychain_use", false)

	// Map ALOGIN_* env vars
	v.SetEnvPrefix("ALOGIN")
	v.AutomaticEnv()

	// Optional config file
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(filepath.Join(configHome, "alogin"))
	_ = v.ReadInConfig() // ignore missing file

	cfg := &Config{
		DataDir:          v.GetString("data_dir"),
		ConfigDir:        v.GetString("config_dir"),
		LogLevel:         v.GetInt("log_level"),
		Lang:             v.GetString("lang"),
		SSHOpt:           v.GetString("sshopt"),
		SSHCmd:           v.GetString("sshcmd"),
		KeychainUse:      v.GetBool("keychain_use"),
		PreferredHost:    v.GetString("preferred_host"),
		DefaultGW:        v.GetString("default_gw"),
		DefaultTermTheme: v.GetString("default_term_theme"),
		LegacyRoot:       os.Getenv("ALOGIN_ROOT"),
	}

	// ALOGIN_DB / ALOGIN_CONFIG overrides
	if dbPath := os.Getenv("ALOGIN_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	} else {
		cfg.DBPath = filepath.Join(cfg.DataDir, "alogin.db")
	}
	cfg.VaultPath = filepath.Join(cfg.DataDir, "vault.age")
	cfg.LogFile = filepath.Join(cfg.DataDir, "alogin.log")

	return cfg, nil
}

// EnsureDirs creates all required directories.
func (c *Config) EnsureDirs() error {
	for _, dir := range []string{c.DataDir, c.ConfigDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func xdgDataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func xdgConfigHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}
