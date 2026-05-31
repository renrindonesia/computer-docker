// Package config loads runtime config from a .env file and the environment.
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings.
type Config struct {
	Addr           string
	APIKey         string
	FSRoot         string
	ExecTimeout    time.Duration
	ExecMaxTimeout time.Duration
}

// Load reads .env (if present) then env vars, with sensible defaults.
func Load(envPath string) Config {
	loadDotEnv(envPath)
	return Config{
		Addr:           envOr("ADDR", ":8080"),
		APIKey:         os.Getenv("API_KEY"),
		FSRoot:         envOr("FS_ROOT", "/opt/data"),
		ExecTimeout:    secs("EXEC_TIMEOUT_SEC", 30),
		ExecMaxTimeout: secs("EXEC_MAX_TIMEOUT_SEC", 300),
	}
}

// loadDotEnv parses KEY=VALUE lines, ignoring comments. Existing env wins.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func secs(key string, def int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(def) * time.Second
}
