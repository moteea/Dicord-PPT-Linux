package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/grafov/evdev"
)

const (
	defaultShortcut = "shift+equal"
	defaultDisplay  = ":0"
	eventKey        = 0x01
)

type config struct {
	DeviceName      string `json:"DEVICE_NAME"`
	DevicePath      string `json:"DEVICE_PATH"`
	PTTKey          string `json:"PTT_KEY"`
	PTTCode         uint16 `json:"PTT_CODE"`
	DiscordShortcut string `json:"DISCORD_SHORTCUT"`
	Display         string `json:"DISPLAY"`
}

type shortcutOverride struct {
	DiscordShortcut string `json:"DISCORD_SHORTCUT"`
}

type configPaths struct {
	dir              string
	staticConfig     string
	runtimeConfig    string
	shortcutOverride string
}

type candidateDevice struct {
	device *evdev.InputDevice
	path   string
	name   string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "discord-ptt-go:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return nil
	}

	switch args[0] {
	case "setup":
		return runSetup(args[1:])
	case "detect":
		return runDetect(args[1:])
	case "debug-detect":
		return runDebugDetect(args[1:])
	case "daemon":
		return runDaemon(args[1:])
	case "print-config":
		return runPrintConfig(args[1:])
	case "help", "--help", "-h":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	configDir := fs.String("config-dir", "", "directory for config/state")
	timeoutSecs := fs.Int("timeout-secs", 30, "button detection timeout in seconds")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	shortcut, err := promptShortcut()
	if err != nil {
		return err
	}

	paths := resolveConfigPaths(*configDir)
	fmt.Println("Trying automatic device/button detection first...")
	return autoDetectAndSave(paths, *timeoutSecs, shortcut)
}

func runDetect(args []string) error {
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	configDir := fs.String("config-dir", "", "directory for config/state")
	timeoutSecs := fs.Int("timeout-secs", 30, "button detection timeout in seconds")
	shortcut := fs.String("shortcut", defaultShortcut, "Discord push-to-talk shortcut")
	devicePath := fs.String("device-path", "", "specific device path to monitor")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	selectedPath := strings.TrimSpace(*devicePath)
	selectedName := ""
	if selectedPath == "" {
		var err error
		selectedPath, selectedName, err = chooseInputDevice()
		if err != nil {
			return err
		}
	} else if name, err := deviceNameFromSysfs(selectedPath); err == nil {
		selectedName = name
	}

	return detectAndSave(resolveConfigPaths(*configDir), *timeoutSecs, normalizeShortcut(*shortcut), selectedPath, selectedName)
}

func runDebugDetect(args []string) error {
	fs := flag.NewFlagSet("debug-detect", flag.ContinueOnError)
	timeoutSecs := fs.Int("timeout-secs", 30, "button detection timeout in seconds")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	devices, err := openCandidateDevices(true)
	if err != nil {
		return err
	}
	defer closeDevices(devices)

	fmt.Printf("Watching %d candidate device(s) for %d seconds...\n", len(devices), *timeoutSecs)
	timeout := time.Duration(*timeoutSecs) * time.Second
	events := startDeviceReaders(devices)
	deadline := time.After(timeout)
	for {
		select {
		case evt := <-events:
			fmt.Printf("device=%s path=%s code=%d value=%d\n", evt.device.name, evt.device.path, evt.event.Code, evt.event.Value)
		case <-deadline:
			return nil
		}
	}
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	configDir := fs.String("config-dir", "", "directory for config/state")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths := resolveConfigPaths(*configDir)
	cfg, err := loadConfig(paths)
	if err != nil {
		return err
	}
	if cfg.DevicePath == "" {
		return errors.New("missing DEVICE_PATH in config")
	}

	devicePath := stableDevicePath(cfg.DevicePath, cfg.DeviceName)
	device, err := evdev.Open(devicePath)
	if err != nil {
		return fmt.Errorf("open device %s: %w", devicePath, err)
	}
	defer device.File.Close()

	fmt.Fprintf(os.Stderr, "listening on %s, button %d, shortcut %s\n", devicePath, cfg.PTTCode, cfg.DiscordShortcut)

	for {
		event, err := device.ReadOne()
		if err != nil {
			return err
		}
		if event.Type != eventKey || event.Code != cfg.PTTCode {
			continue
		}

		current, err := loadConfig(paths)
		if err != nil {
			return err
		}
		switch event.Value {
		case 1:
			if err := sendShortcut(current.DiscordShortcut, current.Display, true); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		case 0:
			if err := sendShortcut(current.DiscordShortcut, current.Display, false); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func runPrintConfig(args []string) error {
	fs := flag.NewFlagSet("print-config", flag.ContinueOnError)
	configDir := fs.String("config-dir", "", "directory for config/state")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig(resolveConfigPaths(*configDir))
	if err != nil {
		return err
	}
	output, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

func printHelp() {
	fmt.Println("Discord PTT helper for Linux")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  discord-ptt-go setup [--config-dir DIR] [--timeout-secs N]")
	fmt.Println("  discord-ptt-go detect [--config-dir DIR] [--timeout-secs N] [--shortcut KEY] [--device-path PATH]")
	fmt.Println("  discord-ptt-go debug-detect [--timeout-secs N]")
	fmt.Println("  discord-ptt-go daemon [--config-dir DIR]")
	fmt.Println("  discord-ptt-go print-config [--config-dir DIR]")
}

func resolveConfigPaths(dir string) configPaths {
	if strings.TrimSpace(dir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		dir = filepath.Join(cwd, "state")
	}

	return configPaths{
		dir:              dir,
		staticConfig:     filepath.Join(dir, "config.json"),
		runtimeConfig:    filepath.Join(dir, "config_detected.json"),
		shortcutOverride: filepath.Join(dir, "shortcut_override.json"),
	}
}

func normalizeShortcut(value string) string {
	clean := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	if clean == "" || clean == "+" || clean == "plus" {
		return defaultShortcut
	}

	parts := strings.Split(clean, "+")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		switch part {
		case "=":
			part = "equal"
		case "esc":
			part = "escape"
		}
		normalized = append(normalized, part)
	}
	if len(normalized) == 0 {
		return defaultShortcut
	}
	return strings.Join(normalized, "+")
}

func normalizeDisplay(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultDisplay
	}
	return value
}

func loadConfig(paths configPaths) (config, error) {
	path := paths.runtimeConfig
	if _, err := os.Stat(path); err != nil {
		path = paths.staticConfig
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.DiscordShortcut = normalizeShortcut(cfg.DiscordShortcut)
	cfg.Display = normalizeDisplay(cfg.Display)

	if data, err := os.ReadFile(paths.shortcutOverride); err == nil {
		var override shortcutOverride
		if json.Unmarshal(data, &override) == nil && strings.TrimSpace(override.DiscordShortcut) != "" {
			cfg.DiscordShortcut = normalizeShortcut(override.DiscordShortcut)
		}
	}

	return cfg, nil
}

func saveDetectedConfig(paths configPaths, cfg config) error {
	if err := os.MkdirAll(paths.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.runtimeConfig, append(data, '\n'), 0o600)
}

func saveShortcutOverride(paths configPaths, shortcut string) error {
	if err := os.MkdirAll(paths.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(shortcutOverride{DiscordShortcut: normalizeShortcut(shortcut)}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.shortcutOverride, append(data, '\n'), 0o600)
}

func promptShortcut() (string, error) {
	fmt.Print("Discord Push-to-Talk shortcut [Shift + =]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultShortcut, nil
	}
	return normalizeShortcut(input), nil
}

func detectAndSave(paths configPaths, timeoutSecs int, shortcut, devicePath, deviceName string) error {
	devices, err := openSpecificDevice(devicePath, deviceName)
	if err != nil {
		return err
	}
	defer closeDevices(devices)

	fmt.Printf("Discord shortcut to save: %s\n", shortcut)
	fmt.Printf("Monitoring device: %s (%s)\n", devices[0].name, devices[0].path)
	fmt.Printf("Press the mouse button you want to use for Discord Push-to-Talk within %d seconds...\n", timeoutSecs)

	event, err := waitForButtonPress(devices[0].device, time.Duration(timeoutSecs)*time.Second)
	if err != nil {
		return err
	}

	cfg := config{
		DeviceName:      devices[0].name,
		DevicePath:      stableDevicePath(devices[0].path, devices[0].name),
		PTTKey:          fmt.Sprintf("BTN_%d", event.Code),
		PTTCode:         event.Code,
		DiscordShortcut: normalizeShortcut(shortcut),
		Display:         defaultDisplay,
	}
	if err := saveDetectedConfig(paths, cfg); err != nil {
		return err
	}
	if err := saveShortcutOverride(paths, cfg.DiscordShortcut); err != nil {
		return err
	}

	fmt.Printf("Saved runtime config to %s\n", paths.runtimeConfig)
	fmt.Printf("Saved shortcut override to %s\n", paths.shortcutOverride)
	fmt.Printf("Device: %s\n", cfg.DeviceName)
	fmt.Printf("Button: %s (%d)\n", cfg.PTTKey, cfg.PTTCode)
	fmt.Printf("Shortcut: %s\n", cfg.DiscordShortcut)
	fmt.Println("Set the same shortcut in Discord Voice & Video.")
	return nil
}

func autoDetectAndSave(paths configPaths, timeoutSecs int, shortcut string) error {
	devices, err := openCandidateDevices(false)
	if err != nil {
		return err
	}
	defer closeDevices(devices)

	fmt.Printf("Discord shortcut to save: %s\n", shortcut)
	fmt.Printf("Press the mouse button you want to use for Discord Push-to-Talk within %d seconds...\n", timeoutSecs)
	timeout := time.Duration(timeoutSecs) * time.Second
	event, err := waitForAnyButtonPress(devices, timeout)
	if err != nil {
		return err
	}

	cfg := config{
		DeviceName:      event.device.name,
		DevicePath:      stableDevicePath(event.device.path, event.device.name),
		PTTKey:          fmt.Sprintf("BTN_%d", event.event.Code),
		PTTCode:         event.event.Code,
		DiscordShortcut: normalizeShortcut(shortcut),
		Display:         defaultDisplay,
	}
	if err := saveDetectedConfig(paths, cfg); err != nil {
		return err
	}
	if err := saveShortcutOverride(paths, cfg.DiscordShortcut); err != nil {
		return err
	}

	fmt.Printf("Auto-detected device: %s (%s)\n", cfg.DeviceName, cfg.DevicePath)
	fmt.Printf("Button: %s (%d)\n", cfg.PTTKey, cfg.PTTCode)
	fmt.Printf("Saved runtime config to %s\n", paths.runtimeConfig)
	fmt.Printf("Saved shortcut override to %s\n", paths.shortcutOverride)
	fmt.Println("Set the same shortcut in Discord Voice & Video.")
	return nil
}

func openCandidateDevices(verbose bool) ([]candidateDevice, error) {
	entries := prioritizedDevicePaths()

	var devices []candidateDevice
	for _, path := range entries {
		device, err := evdev.Open(path)
		if err != nil {
			continue
		}

		name := filepath.Base(path)
		if resolvedName, err := deviceNameFromSysfs(path); err == nil && resolvedName != "" {
			name = resolvedName
		}

		devices = append(devices, candidateDevice{
			device: device,
			path:   path,
			name:   name,
		})
		if verbose {
			fmt.Printf("candidate device: %s (%s)\n", name, path)
		}
	}

	if len(devices) == 0 {
		return nil, errors.New("no accessible input devices found")
	}
	return devices, nil
}

func openSpecificDevice(path, name string) ([]candidateDevice, error) {
	device, err := evdev.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open device %s: %w", path, err)
	}

	if name == "" {
		name = filepath.Base(path)
	}

	fmt.Printf("candidate device: %s (%s)\n", name, path)
	return []candidateDevice{{
		device: device,
		path:   path,
		name:   name,
	}}, nil
}

func prioritizedDevicePaths() []string {
	patterns := []string{
		"/dev/input/by-id/*event-mouse",
		"/dev/input/event*",
	}

	seen := make(map[string]bool)
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if seen[match] {
				continue
			}
			seen[match] = true
			paths = append(paths, match)
		}
	}
	return paths
}

func chooseInputDevice() (string, string, error) {
	devices, err := listSelectableDevices()
	if err != nil {
		return "", "", err
	}
	if len(devices) == 0 {
		return "", "", errors.New("no selectable mouse-like devices found")
	}

	fmt.Println("Choose the mouse/input device to monitor:")
	for index, device := range devices {
		fmt.Printf("  %d. %s (%s)\n", index+1, device.name, device.path)
	}
	fmt.Print("Selection [1]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return devices[0].path, devices[0].name, nil
	}

	var choice int
	if _, err := fmt.Sscanf(input, "%d", &choice); err != nil || choice < 1 || choice > len(devices) {
		return "", "", fmt.Errorf("invalid selection %q", input)
	}

	device := devices[choice-1]
	return device.path, device.name, nil
}

func listSelectableDevices() ([]candidateDevice, error) {
	paths := prioritizedDevicePaths()
	seen := make(map[string]bool)
	var devices []candidateDevice
	for _, path := range paths {
		if !strings.Contains(path, "event-mouse") && !strings.Contains(filepath.Base(path), "event") {
			continue
		}

		name := filepath.Base(path)
		if resolvedName, err := deviceNameFromSysfs(path); err == nil && resolvedName != "" {
			name = resolvedName
		}

		key := path + "|" + name
		if seen[key] {
			continue
		}
		seen[key] = true

		lowerName := strings.ToLower(name)
		if strings.Contains(lowerName, "power button") ||
			strings.Contains(lowerName, "audio") ||
			strings.Contains(lowerName, "video bus") ||
			strings.Contains(lowerName, "hdmi") {
			continue
		}

		devices = append(devices, candidateDevice{
			path: path,
			name: name,
		})
	}
	return devices, nil
}

func closeDevices(devices []candidateDevice) {
	for _, device := range devices {
		if device.device != nil && device.device.File != nil {
			_ = device.device.File.Close()
		}
	}
}

func deviceNameFromSysfs(devicePath string) (string, error) {
	base := filepath.Base(devicePath)
	namePath := filepath.Join("/sys/class/input", base, "device", "name")
	data, err := os.ReadFile(namePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func stableDevicePath(devicePath, deviceName string) string {
	if devicePath == "" {
		return devicePath
	}

	realPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return devicePath
	}

	entries, err := os.ReadDir("/dev/input/by-id")
	if err != nil {
		return devicePath
	}

	slug := strings.ToLower(strings.ReplaceAll(deviceName, " ", "_"))
	var preferred []string
	var fallback []string
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, "event") {
			continue
		}

		fullPath := filepath.Join("/dev/input/by-id", entry.Name())
		candidateRealPath, err := filepath.EvalSymlinks(fullPath)
		if err != nil || candidateRealPath != realPath {
			continue
		}

		if slug != "" && strings.Contains(name, slug) && strings.Contains(name, "event-mouse") {
			preferred = append(preferred, fullPath)
		} else {
			fallback = append(fallback, fullPath)
		}
	}

	if len(preferred) > 0 {
		return preferred[0]
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return devicePath
}

func waitForButtonPress(device *evdev.InputDevice, timeout time.Duration) (*evdev.InputEvent, error) {
	type result struct {
		event *evdev.InputEvent
		err   error
	}

	resultCh := make(chan result, 1)
	go func() {
		for {
			event, err := device.ReadOne()
			if err != nil {
				resultCh <- result{err: err}
				return
			}
			if event.Type == eventKey && event.Value == 1 {
				resultCh <- result{event: event}
				return
			}
		}
	}()

	select {
	case result := <-resultCh:
		return result.event, result.err
	case <-time.After(timeout):
		return nil, errors.New("timed out waiting for a button press")
	}
}

type detectedEvent struct {
	device candidateDevice
	event  *evdev.InputEvent
}

func startDeviceReaders(devices []candidateDevice) chan detectedEvent {
	out := make(chan detectedEvent, 32)
	for _, device := range devices {
		device := device
		go func() {
			for {
				event, err := device.device.ReadOne()
				if err != nil {
					return
				}
				if event.Type == eventKey {
					out <- detectedEvent{device: device, event: event}
				}
			}
		}()
	}
	return out
}

func waitForAnyButtonPress(devices []candidateDevice, timeout time.Duration) (detectedEvent, error) {
	events := startDeviceReaders(devices)
	deadline := time.After(timeout)
	for {
		select {
		case evt := <-events:
			if evt.event.Value == 1 {
				return evt, nil
			}
		case <-deadline:
			return detectedEvent{}, errors.New("timed out waiting for a button press")
		}
	}
}

func sendShortcut(shortcut, display string, press bool) error {
	tokens := strings.Split(normalizeShortcut(shortcut), "+")
	if len(tokens) == 0 {
		return errors.New("shortcut cannot be empty")
	}

	if !press {
		for left, right := 0, len(tokens)-1; left < right; left, right = left+1, right-1 {
			tokens[left], tokens[right] = tokens[right], tokens[left]
		}
	}

	action := "keydown"
	if !press {
		action = "keyup"
	}

	for _, token := range tokens {
		command := exec.Command("xdotool", action, token)
		command.Env = append(os.Environ(), "DISPLAY="+normalizeDisplay(display))
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("xdotool %s %s failed: %w (%s)", action, token, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}
