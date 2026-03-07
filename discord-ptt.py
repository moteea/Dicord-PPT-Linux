#!/usr/bin/env python3
import json
import os
import subprocess
import sys

from evdev import InputDevice, ecodes, list_devices


CONFIG_PATHS = (
    os.path.expanduser("~/.config/ptt/config_detected.json"),
    os.path.expanduser("~/.config/ptt/config.json"),
)
DEFAULT_SHORTCUT = "shift+equal"
DEFAULT_DISPLAY = ":0"
XDOTOOL_BIN = os.environ.get("XDOTOOL_BIN", "xdotool")


def normalize_shortcut(value):
    shortcut = str(value or DEFAULT_SHORTCUT).strip().lower().replace(" ", "")
    if shortcut in ("", "+", "plus"):
        return DEFAULT_SHORTCUT
    return shortcut


def load_config():
    for path in CONFIG_PATHS:
        if not os.path.exists(path):
            continue

        with open(path, "r", encoding="utf-8") as handle:
            cfg = json.load(handle)

        cfg["DISCORD_SHORTCUT"] = normalize_shortcut(cfg.get("DISCORD_SHORTCUT"))
        cfg["DISPLAY"] = str(cfg.get("DISPLAY", DEFAULT_DISPLAY))

        if isinstance(cfg.get("PTT_KEY"), str):
            key_name = cfg["PTT_KEY"].strip()
            if key_name.isdigit():
                cfg["PTT_KEY"] = int(key_name)
            else:
                cfg["PTT_KEY"] = getattr(ecodes, key_name, key_name)

        return cfg

    raise FileNotFoundError(
        "no config found at ~/.config/ptt/config_detected.json or ~/.config/ptt/config.json"
    )


def resolve_ptt_code(cfg):
    if cfg.get("PTT_CODE") not in (None, ""):
        return int(cfg["PTT_CODE"])

    ptt_key = cfg.get("PTT_KEY")
    if isinstance(ptt_key, int):
        return ptt_key
    if isinstance(ptt_key, str):
        if ptt_key.isdigit():
            return int(ptt_key)
        resolved = getattr(ecodes, ptt_key, None)
        if resolved is None:
            raise ValueError(f"unknown PTT_KEY: {ptt_key}")
        return int(resolved)

    raise ValueError("config must define PTT_CODE or PTT_KEY")


def device_supports_code(device, ptt_code):
    caps = device.capabilities()
    return ecodes.EV_KEY in caps and ptt_code in caps[ecodes.EV_KEY]


def device_score(device, name_hint):
    name = (device.name or "").lower()
    score = 0
    if name_hint and name_hint in name:
        score += 20
    if "mouse" in name:
        score += 2
    if "keyboard" in name:
        score -= 5
    if "consumer" in name:
        score -= 5
    return score


def find_mouse_device(cfg, ptt_code):
    device_path = str(cfg.get("DEVICE_PATH", "")).strip()
    if device_path and os.path.exists(device_path):
        try:
            device = InputDevice(device_path)
            if device_supports_code(device, ptt_code):
                return device
            print(
                f"discord-ptt: configured device does not expose button code {ptt_code}: {device_path}",
                file=sys.stderr,
            )
        except Exception as exc:
            print(f"discord-ptt: failed to use configured device {device_path}: {exc}", file=sys.stderr)

    name_hint = str(cfg.get("DEVICE_NAME", "")).strip().lower()
    candidates = []

    for path in list_devices():
        try:
            device = InputDevice(path)
            if device_supports_code(device, ptt_code):
                candidates.append(device)
        except Exception:
            continue

    if not candidates:
        raise RuntimeError(f"no input device exposing button code {ptt_code} was found")

    return max(candidates, key=lambda device: device_score(device, name_hint))


def send(shortcut, display, pressed):
    env = os.environ.copy()
    env["DISPLAY"] = display
    cmd = "keydown" if pressed else "keyup"
    subprocess.run(
        [XDOTOOL_BIN, cmd, shortcut],
        check=False,
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def main():
    try:
        cfg = load_config()
        ptt_code = resolve_ptt_code(cfg)
        device = find_mouse_device(cfg, ptt_code)
    except Exception as exc:
        print(f"discord-ptt: startup failed: {exc}", file=sys.stderr)
        return 1

    shortcut = str(cfg["DISCORD_SHORTCUT"])
    display = str(cfg.get("DISPLAY", DEFAULT_DISPLAY))
    ptt_label = cfg.get("PTT_KEY", ptt_code)
    is_pressed = False

    print(
        f"discord-ptt: listening on {device.path} ({device.name}), button {ptt_label}, shortcut {shortcut}",
        file=sys.stderr,
    )

    for event in device.read_loop():
        if event.type != ecodes.EV_KEY or event.code != ptt_code:
            continue

        if event.value == 1 and not is_pressed:
            send(shortcut, display, True)
            is_pressed = True
        elif event.value == 0 and is_pressed:
            send(shortcut, display, False)
            is_pressed = False

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
