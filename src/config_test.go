package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// env turns a map into a getenv function, so tests never touch real
// environment variables.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Port:           "8000",
		Backends:       demoBackends,
		HealthInterval: 2 * time.Second,
		HealthTimeout:  1 * time.Second,
		Demo:           true,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{
		"LB_PORT":            "9090",
		"LB_BACKENDS":        " http://app1:8080 , https://app2.internal/ ,",
		"LB_HEALTH_INTERVAL": "5s",
		"LB_HEALTH_TIMEOUT":  "500ms",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Port:           "9090",
		Backends:       []string{"http://app1:8080", "https://app2.internal"},
		HealthInterval: 5 * time.Second,
		HealthTimeout:  500 * time.Millisecond,
		Demo:           false,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	tests := []struct {
		name    string
		vars    map[string]string
		wantErr string // a piece of the error message
	}{
		{"port not a number", map[string]string{"LB_PORT": "abc"}, "LB_PORT"},
		{"port out of range", map[string]string{"LB_PORT": "70000"}, "LB_PORT"},
		{"backend missing scheme", map[string]string{"LB_BACKENDS": "localhost:9001"}, "http://"},
		{"backend missing host", map[string]string{"LB_BACKENDS": "http://"}, "no host"},
		{"backends only commas", map[string]string{"LB_BACKENDS": ", ,"}, "no addresses"},
		{"interval not a duration", map[string]string{"LB_HEALTH_INTERVAL": "2"}, "LB_HEALTH_INTERVAL"},
		{"timeout negative", map[string]string{"LB_HEALTH_TIMEOUT": "-1s"}, "LB_HEALTH_TIMEOUT"},
		{"timeout not shorter than interval", map[string]string{"LB_HEALTH_INTERVAL": "1s", "LB_HEALTH_TIMEOUT": "1s"}, "shorter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(env(tt.vars))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should mention %q", err, tt.wantErr)
			}
		})
	}
}
