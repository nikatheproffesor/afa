package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences", "settings.json")
	initial, err := Load(path)
	if err != nil || initial != Default() {
		t.Fatalf("missing settings got %v, %v", initial, err)
	}
	changed := initial
	changed.Theme = "dark"
	changed.StartInTray = true
	changed.TimeoutSeconds = 60
	if err := Save(path, changed); err != nil {
		t.Fatal(err)
	}
	other, err := Load(path)
	if err != nil || other != changed {
		t.Fatalf("saved settings got %v, %v", other, err)
	}
	bad := changed
	bad.TimeoutSeconds = 120
	if Save(path, bad) == nil {
		t.Fatal("saved settings with invalid timeout")
	}
	other, err = Load(path)
	if err != nil || other != changed {
		t.Fatalf("invalid write changed existing values: %v, %v", other, err)
	}
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	other, err = Load(path)
	if err != nil || other != Default() {
		t.Fatalf("reset got %v, %v", other, err)
	}
}

func TestSettingsRecoveryAndMigration(t *testing.T) {
	cases := []struct {
		name string
		body string
		bad  bool
	}{
		{"unversioned", `{"theme":"dark"}`, false},
		{"version zero", `{"version":0,"theme":"dark"}`, false},
		{"malformed", `{"theme":`, true},
		{"nonobject", `null`, true},
		{"future", `{"version":2,"theme":"dark"}`, true},
		{"out of bounds", `{"timeout_seconds":2}`, true},
		{"oversize", strings.Repeat("x", MaxFileBytes+1), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := Load(path)
			if tc.bad {
				if err == nil || got != Default() {
					t.Fatalf("expected safe defaults and error, got %v, %v", got, err)
				}
			} else if err != nil || got.Theme != "dark" || got.Version != Schema || got.Language != "tr" {
				t.Fatalf("expected migrated settings, got %v, %v", got, err)
			}
			raw, err := os.ReadFile(path)
			if err != nil || string(raw) != tc.body {
				t.Fatal("load modified the original settings")
			}
		})
	}
}
