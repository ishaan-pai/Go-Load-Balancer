package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	Backends       []string
	HealthInterval time.Duration
	HealthTimeout  time.Duration
	Demo           bool
}

var demoBackends = []string{
	"http://localhost:9001",
	"http://localhost:9002",
	"http://localhost:9003",
}

func loadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:           "8000",
		HealthInterval: 2 * time.Second,
		HealthTimeout:  1 * time.Second,
	}

	if v := getenv("LB_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			return Config{}, fmt.Errorf("LB_PORT: %q is not a port number between 1 and 65535", v)
		}
		cfg.Port = v
	}

	if v := getenv("LB_BACKENDS"); v != "" {
		for _, raw := range strings.Split(v, ",") {
			addr := strings.TrimSpace(raw)
			if addr == "" {
				continue
			}
			if err := validateBackendURL(addr); err != nil {
				return Config{}, fmt.Errorf("LB_BACKENDS: %w", err)
			}
			cfg.Backends = append(cfg.Backends, strings.TrimRight(addr, "/"))
		}
		if len(cfg.Backends) == 0 {
			return Config{}, fmt.Errorf("LB_BACKENDS: %q contains no addresses", v)
		}
	} else {
		cfg.Demo = true
		cfg.Backends = demoBackends
	}

	var err error
	if cfg.HealthInterval, err = durationEnv(getenv, "LB_HEALTH_INTERVAL", cfg.HealthInterval); err != nil {
		return Config{}, err
	}
	if cfg.HealthTimeout, err = durationEnv(getenv, "LB_HEALTH_TIMEOUT", cfg.HealthTimeout); err != nil {
		return Config{}, err
	}
	if cfg.HealthTimeout >= cfg.HealthInterval {
		return Config{}, fmt.Errorf("LB_HEALTH_TIMEOUT (%s) must be shorter than LB_HEALTH_INTERVAL (%s)",
			cfg.HealthTimeout, cfg.HealthInterval)
	}

	return cfg, nil
}

func validateBackendURL(addr string) error {
	u, err := url.Parse(addr)
	if err != nil {
		return fmt.Errorf("%q is not a valid URL: %w", addr, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%q must start with http:// or https://", addr)
	}
	if u.Host == "" {
		return fmt.Errorf("%q has no host", addr)
	}
	return nil
}

func durationEnv(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive duration like 500ms or 2s", key, v)
	}
	return d, nil
}
