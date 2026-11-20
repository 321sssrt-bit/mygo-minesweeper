package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrefsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dev.mygo.minesweeper", "prefs.json")
	for _, want := range []prefs{
		{DarkMode: true, Motion: motionStandard, SoundEnabled: true, Volume: 100},
		{Motion: motionReduced, AllowQuestion: true},
	} {
		if err := savePrefs(dir, want); err != nil {
			t.Fatal(err)
		}
		if got := loadPrefs(dir); got != want {
			t.Fatalf("loaded %+v, want %+v", got, want)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		keys := []string{"darkMode", "motion", "soundEnabled", "volume", "allowQuestion"}
		if len(fields) != len(keys) {
			t.Fatalf("expected exactly five preferences, got %s", data)
		}
		for _, key := range keys {
			if _, ok := fields[key]; !ok {
				t.Fatalf("missing %q in %s", key, data)
			}
		}
		t.Logf("saved and restored %s", data)
	}
}

func TestPrefsPartial(t *testing.T) {
	dir := t.TempDir()
	writePrefsFixture(t, dir, `{"soundEnabled":false,"volume":0,"allowQuestion":true}`)
	want := prefs{DarkMode: true, Motion: motionSystem, AllowQuestion: true}
	got := loadPrefs(dir)
	if got != want {
		t.Fatalf("loaded %+v, want %+v", got, want)
	}
	t.Logf("partial file restored %+v", got)
}

func TestPrefsFallback(t *testing.T) {
	want := prefs{DarkMode: true, Motion: motionSystem, SoundEnabled: true, Volume: 65}
	for _, tc := range []struct{ name, data string }{
		{name: "missing_file"},
		{name: "unreadable_path"},
		{name: "empty_file"},
		{name: "invalid_encoding", data: "{\"darkMode\":false,\"unknown\":\"\xff\"}"},
		{name: "truncated_json", data: `{"darkMode":false,`},
		{name: "wrong_field_type", data: `{"darkMode":false,"soundEnabled":"no"}`},
		{name: "unknown_motion", data: `{"motion":"fast"}`},
		{name: "invalid_volume", data: `{"volume":101}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			switch tc.name {
			case "missing_file":
			case "unreadable_path":
				if err := os.MkdirAll(prefsPath(dir), 0o700); err != nil {
					t.Fatal(err)
				}
			default:
				writePrefsFixture(t, dir, tc.data)
			}
			got := loadPrefs(dir)
			if got != want {
				t.Fatalf("loaded %+v, want %+v", got, want)
			}
			t.Logf("restored defaults %+v", got)
		})
	}
}

func TestPrefsReplaceFailure(t *testing.T) {
	dir := t.TempDir()
	const old = `{"darkMode":true,"motion":"system","soundEnabled":true,"volume":65,"allowQuestion":false}`
	writePrefsFixture(t, dir, old)
	path := prefsPath(dir)
	updated := prefs{Motion: motionReduced, Volume: 25, AllowQuestion: true}
	replaceErr := errors.New("replacement failed")
	err := writePrefs(path, updated, func(src, dst string) error {
		if dst != path || src == dst || filepath.Dir(src) != filepath.Dir(dst) {
			t.Fatalf("expected replacement from a separate file in the same directory")
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		var staged prefs
		if err := json.Unmarshal(data, &staged); err != nil || staged != updated {
			t.Fatalf("staged %s, want %+v (decode error: %v)", data, updated, err)
		}
		return replaceErr
	})
	if !errors.Is(err, replaceErr) {
		t.Fatalf("save returned %v, want %v", err, replaceErr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != old {
		t.Fatalf("old file changed to %s", data)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("unexpected directory contents after failed save: %v", entries)
	}
	t.Logf("replacement failed, previous file retained byte for byte: %s; no temporary file remains", data)
}

func writePrefsFixture(t *testing.T, dir, data string) {
	t.Helper()
	path := prefsPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
