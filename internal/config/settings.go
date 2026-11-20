// Package config stores non-secret desktop preferences in the user config directory.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	Schema       = 1
	MaxFileBytes = 16 << 10
)

type Settings struct {
	Version        int    `json:"version"`
	Theme          string `json:"theme"`
	Language       string `json:"language"`
	StartInTray    bool   `json:"start_in_tray"`
	Monitoring     bool   `json:"monitoring"`
	Notifications  bool   `json:"notifications"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func Default() Settings {
	return Settings{Version: Schema, Theme: "light", Language: "tr", TimeoutSeconds: 10}
}

func UserPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "AFA", "settings.json"), nil
}

func (s Settings) Validate() error {
	if s.Version != Schema {
		return errors.New("unsupported settings version")
	}
	if s.Theme != "light" && s.Theme != "dark" {
		return errors.New("invalid theme")
	}
	if s.Language != "tr" && s.Language != "en" {
		return errors.New("invalid language")
	}
	if s.TimeoutSeconds < 3 || s.TimeoutSeconds > 60 {
		return errors.New("invalid diagnostic timeout")
	}
	return nil
}

// Load returns defaults together with an error for invalid files. It only reads
// the file, so a failed load never overwrites the user's last settings.
func Load(path string) (Settings, error) {
	settings := Default()
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return settings, err
	}
	if len(data) > MaxFileBytes {
		return settings, errors.New("settings file exceeds size limit")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return settings, errors.New("settings file must contain a JSON object")
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return Default(), errors.New("invalid settings JSON")
	}
	if settings.Version == 0 {
		settings.Version = Schema
	}
	if err := settings.Validate(); err != nil {
		return Default(), err
	}
	return settings, nil
}

// Save flushes a sibling temporary file before replacing the destination.
func Save(path string, settings Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".settings-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(settings); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return replace(file.Name(), path)
}
