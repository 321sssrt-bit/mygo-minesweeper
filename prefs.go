package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"unicode/utf8"
)

type motionMode string

const (
	motionSystem   motionMode = "system"
	motionStandard motionMode = "standard"
	motionReduced  motionMode = "reduced"
)

type prefs struct {
	DarkMode      bool       `json:"darkMode"`
	Motion        motionMode `json:"motion"`
	SoundEnabled  bool       `json:"soundEnabled"`
	Volume        int        `json:"volume"`
	AllowQuestion bool       `json:"allowQuestion"`
}

func defaultPrefs() prefs {
	return prefs{
		DarkMode:     true,
		Motion:       motionSystem,
		SoundEnabled: true,
		Volume:       65,
	}
}

func normalizePrefs(p prefs) prefs {
	switch p.Motion {
	case motionSystem, motionStandard, motionReduced:
	default:
		p.Motion = motionSystem
	}
	p.Volume = max(0, min(100, p.Volume))
	return p
}

func (m motionMode) reducedMotion(systemReduce bool) bool {
	switch m {
	case motionStandard:
		return false
	case motionReduced:
		return true
	default:
		return systemReduce
	}
}

// configDir is the root returned by os.UserConfigDir.
func prefsPath(configDir string) string {
	return filepath.Join(configDir, "dev.mygo.minesweeper", "prefs.json")
}

func loadPrefs(configDir string) prefs {
	p := defaultPrefs()
	data, err := os.ReadFile(prefsPath(configDir))
	if err != nil || !utf8.Valid(data) {
		return p
	}
	// Decode over the defaults so missing fields and explicit zero values differ.
	if err := json.Unmarshal(data, &p); err != nil || p.Volume < 0 || p.Volume > 100 {
		return defaultPrefs()
	}
	switch p.Motion {
	case motionSystem, motionStandard, motionReduced:
		return p
	default:
		return defaultPrefs()
	}
}

func savePrefs(configDir string, p prefs) error {
	return writePrefs(prefsPath(configDir), p, os.Rename)
}

func writePrefs(path string, p prefs, replace func(string, string) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".prefs-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := json.NewEncoder(tmp).Encode(p); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replace(tmp.Name(), path)
}
