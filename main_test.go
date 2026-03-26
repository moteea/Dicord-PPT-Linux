package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeShortcut(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":              defaultShortcut,
		"plus":          defaultShortcut,
		"Shift + =":     "shift+equal",
		" ctrl + shift + p ": "ctrl+shift+p",
	}

	for input, want := range cases {
		if got := normalizeShortcut(input); got != want {
			t.Fatalf("normalizeShortcut(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLoadConfigPrecedence(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	paths := resolveConfigPaths(tempDir)

	staticCfg := config{
		DeviceName:      "Mouse",
		DevicePath:      "/dev/input/event1",
		PTTKey:          "BTN_276",
		PTTCode:         276,
		DiscordShortcut: "shift+equal",
		Display:         ":0",
	}
	runtimeCfg := staticCfg
	runtimeCfg.DiscordShortcut = "ctrl+shift+p"

	writeJSON(t, paths.staticConfig, staticCfg)
	writeJSON(t, paths.runtimeConfig, runtimeCfg)
	writeJSON(t, paths.shortcutOverride, shortcutOverride{DiscordShortcut: "ctrl+shift+m"})

	got, err := loadConfig(paths)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if got.DiscordShortcut != "ctrl+shift+m" {
		t.Fatalf("shortcut = %q, want ctrl+shift+m", got.DiscordShortcut)
	}
}

func TestSaveDetectedConfigRoundTrip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	paths := resolveConfigPaths(tempDir)
	cfg := config{
		DeviceName:      "Mouse",
		DevicePath:      "/dev/input/by-id/mouse",
		PTTKey:          "BTN_276",
		PTTCode:         276,
		DiscordShortcut: "shift+equal",
		Display:         ":0",
	}

	if err := saveDetectedConfig(paths, cfg); err != nil {
		t.Fatalf("saveDetectedConfig: %v", err)
	}
	if err := saveShortcutOverride(paths, cfg.DiscordShortcut); err != nil {
		t.Fatalf("saveShortcutOverride: %v", err)
	}

	got, err := loadConfig(paths)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if got.PTTCode != cfg.PTTCode || got.DevicePath != cfg.DevicePath {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, cfg)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
