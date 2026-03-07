#!/usr/bin/env python3
import json
import os
import re
import select
import time
from pathlib import Path

import evdev
from evdev import InputDevice, ecodes


CONFIG_DIR = Path(os.path.expanduser("~/.config/ptt"))
STATIC_CONFIG_PATH = CONFIG_DIR / "config.json"
RUNTIME_CONFIG_PATH = CONFIG_DIR / "config_detected.json"
DEFAULT_SHORTCUT = "shift+equal"
DEFAULT_DISPLAY = ":0"


def normalize_shortcut(value):
    shortcut = str(value or DEFAULT_SHORTCUT).strip().lower().replace(" ", "")
    if shortcut in ("", "+", "plus"):
        return DEFAULT_SHORTCUT
    return shortcut


def load_existing_config():
    for path in (RUNTIME_CONFIG_PATH, STATIC_CONFIG_PATH):
        if path.exists():
            with path.open("r", encoding="utf-8") as handle:
                return json.load(handle)
    return {}


def button_name(code):
    key_map = ecodes.bytype.get(ecodes.EV_KEY, {})
    return key_map.get(code, f"BTN_{code}")


def list_input_devices():
    devices = []
    for path in evdev.list_devices():
        try:
            device = InputDevice(path)
            capabilities = device.capabilities()
            if ecodes.EV_KEY not in capabilities:
                continue
            devices.append(device)
        except Exception:
            continue
    return devices


def monitor_devices(timeout=30):
    config = load_existing_config()
    name_hint = str(config.get("DEVICE_NAME", "")).strip().lower()
    devices = list_input_devices()

    if name_hint:
        matching = [device for device in devices if name_hint in (device.name or "").lower()]
        if matching:
            devices = matching

    if not devices:
        raise RuntimeError("no readable input devices with key events were found")

    print("Press the mouse button you want to use for Discord Push-to-Talk.")
    deadline = time.monotonic() + timeout

    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError("timed out waiting for a button press")

        readable, _, _ = select.select(devices, [], [], min(0.5, remaining))
        for device in readable:
            try:
                events = device.read()
            except BlockingIOError:
                continue

            for event in events:
                if event.type != ecodes.EV_KEY or event.value != 1:
                    continue

                return {
                    "DEVICE_NAME": device.name,
                    "DEVICE_PATH": device.path,
                    "PTT_KEY": button_name(event.code),
                    "PTT_CODE": event.code,
                }


def format_ptt_config(config):
    def nix_string(value):
        return json.dumps(str(value))

    lines = [
        "  pttConfig = {",
        f"    DEVICE_NAME = {nix_string(config['DEVICE_NAME'])};",
        f"    DEVICE_PATH = {nix_string(config['DEVICE_PATH'])};",
        f"    PTT_KEY = {nix_string(config['PTT_KEY'])};",
        f'    PTT_CODE = {config["PTT_CODE"]};',
        f"    DISCORD_SHORTCUT = {nix_string(config['DISCORD_SHORTCUT'])};",
        f"    DISPLAY = {nix_string(config['DISPLAY'])};",
        "  };",
    ]
    return "\n".join(lines)


def maybe_update_nix_config(config):
    nix_config_path = os.environ.get("PTT_NIX_CONFIG_PATH", "").strip()
    if not nix_config_path:
        return False

    path = Path(os.path.expanduser(nix_config_path))
    if not path.exists():
        print(f"Nix config path does not exist: {path}")
        return False

    original = path.read_text(encoding="utf-8")
    pattern = re.compile(r"(?ms)^  pttConfig = \{.*?^  \};")
    replacement = format_ptt_config(config)
    updated = pattern.sub(replacement, original, count=1)

    if updated == original:
        print(f"Could not find pttConfig block in {path}")
        return False

    path.write_text(updated, encoding="utf-8")
    print(f"Updated Nix config: {path}")
    return True


def save_runtime_config(config):
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    with RUNTIME_CONFIG_PATH.open("w", encoding="utf-8") as handle:
        json.dump(config, handle, indent=2)
        handle.write("\n")


def main():
    existing = load_existing_config()
    shortcut = normalize_shortcut(os.environ.get("DISCORD_SHORTCUT", existing.get("DISCORD_SHORTCUT")))
    display = str(existing.get("DISPLAY", DEFAULT_DISPLAY))

    detected = monitor_devices()
    detected["DISCORD_SHORTCUT"] = shortcut
    detected["DISPLAY"] = display

    save_runtime_config(detected)
    print(f"Saved runtime config to {RUNTIME_CONFIG_PATH}")

    if not maybe_update_nix_config(detected):
        print("Update your Home Manager pttConfig block with:")
        print(format_ptt_config(detected))


if __name__ == "__main__":
    main()
