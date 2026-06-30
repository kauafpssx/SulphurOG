package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	API struct {
		Port   int    `yaml:"port"`
		APIKey string `yaml:"api_key"`
	} `yaml:"api"`
	Telegram struct {
		APIID       string   `yaml:"api_id"`
		APIHash     string   `yaml:"api_hash"`
		Phone       string   `yaml:"phone"`
		SessionFile string   `yaml:"session_file"`
		Channels    []string `yaml:"channels"`
	} `yaml:"telegram"`
	Supabase struct {
		URL            string `yaml:"url"`
		AnonKey        string `yaml:"anon_key"`
		ServiceRoleKey string `yaml:"service_role_key"`
		Bucket         string `yaml:"bucket"`
	} `yaml:"supabase"`
	Processing struct {
		TempDir            string   `yaml:"temp_dir"`
		PartSizeKB         int      `yaml:"part_size_kb"`
		MaxRetries         int      `yaml:"max_retries"`
		PollInterval       string   `yaml:"poll_interval"`
		Threads            int      `yaml:"threads"`
		ProcessCookies     bool     `yaml:"process_cookies"`
		AllowedExtensions  []string `yaml:"allowed_extensions"`
	} `yaml:"processing"`
	Logging struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"logging"`
}

func LoadConfig(path string) (*Config, error) {
	loadDotEnv()

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer f.Close()

	cfg := &Config{}
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	applyDefaults(cfg)
	expandEnvVars(cfg)

	return cfg, nil
}

func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")

		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func applyDefaults(cfg *Config) {
	if cfg.API.Port == 0 {
		cfg.API.Port = 8080
	}
	if cfg.Processing.TempDir == "" {
		cfg.Processing.TempDir = "/tmp/sulphurog"
	}
	if cfg.Processing.PartSizeKB == 0 {
		cfg.Processing.PartSizeKB = 512
	}
	if cfg.Processing.MaxRetries == 0 {
		cfg.Processing.MaxRetries = 3
	}
	if cfg.Processing.Threads == 0 {
		cfg.Processing.Threads = 16
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
}

func expandEnvVars(cfg *Config) {
	cfg.Telegram.APIID = expandEnv(cfg.Telegram.APIID)
	cfg.Telegram.APIHash = expandEnv(cfg.Telegram.APIHash)
	cfg.Telegram.Phone = expandEnv(cfg.Telegram.Phone)
	cfg.Supabase.URL = expandEnv(cfg.Supabase.URL)
	cfg.Supabase.AnonKey = expandEnv(cfg.Supabase.AnonKey)
	cfg.Supabase.ServiceRoleKey = expandEnv(cfg.Supabase.ServiceRoleKey)
	cfg.Supabase.Bucket = expandEnv(cfg.Supabase.Bucket)
	cfg.API.APIKey = expandEnv(cfg.API.APIKey)

	// Fallback session file: tentar caminhos comuns
	if cfg.Telegram.SessionFile == "" {
		cfg.Telegram.SessionFile = os.Getenv("TG_SESSION_FILE")
	}
	if cfg.Telegram.SessionFile == "" {
		fallbacks := []string{
			"data/session.json",
			"data/session.key",
			"/opt/sulphurog/data/session.json",
			"/opt/sulphurog/data/session.key",
			os.ExpandEnv("$HOME/.gotd/session.json"),
		}
		for _, path := range fallbacks {
			if _, err := os.Stat(path); err == nil {
				cfg.Telegram.SessionFile = path
				break
			}
		}
		if cfg.Telegram.SessionFile == "" {
			cfg.Telegram.SessionFile = "data/session.json" // default
		}
	}

	if cfg.API.APIKey == "" {
		cfg.API.APIKey = os.Getenv("API_KEY")
	}
	if cfg.Telegram.APIID == "" {
		cfg.Telegram.APIID = os.Getenv("TG_API_ID")
	}
	if cfg.Telegram.APIHash == "" {
		cfg.Telegram.APIHash = os.Getenv("TG_API_HASH")
	}
	if cfg.Telegram.Phone == "" {
		cfg.Telegram.Phone = os.Getenv("TG_PHONE")
	}
	if cfg.Supabase.URL == "" {
		cfg.Supabase.URL = os.Getenv("SUPABASE_URL")
	}
	if cfg.Supabase.AnonKey == "" {
		cfg.Supabase.AnonKey = os.Getenv("SUPABASE_ANON_KEY")
	}
	if cfg.Supabase.ServiceRoleKey == "" {
		cfg.Supabase.ServiceRoleKey = os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	}
	if cfg.Supabase.Bucket == "" {
		cfg.Supabase.Bucket = os.Getenv("SUPABASE_BUCKET")
	}
}

func expandEnv(s string) string {
	if !strings.HasPrefix(s, "${") {
		return s
	}
	key := strings.Trim(s, "${}")
	return os.Getenv(key)
}

func (c *Config) TelegramAPIIDInt() int {
	id, _ := strconv.Atoi(c.Telegram.APIID)
	return id
}
