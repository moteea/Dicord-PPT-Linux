#!/usr/bin/env python3
import json
import os
import subprocess
import sys

from evdev import InputDevice, ecodes


CONFIG_PATH = os.path.expanduser("~/.config/ptt/config.json")


def load_config():
    with open(CONFIG_PATH, "r", encoding="utf-8") as handle:
        cfg = json.load(handle)

    required = ["DEVICE_PATH", "PTT_CODE", "DISCORD_SHORTCUT"]
    missing = [key for key in required if not cfg.get(key)]
    if missing:
        raise ValueError(f"missing config keys: {', '.join(missing)}")

    return cfg


def send(shortcut, display, pressed):
    env = os.environ.copy()
    env["DISPLAY"] = display
    cmd = "keydown" if pressed else "keyup"
    subprocess.run(
        ["xdotool", cmd, shortcut],
        check=False,
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def main():
    try:
        cfg = load_config()
        device = InputDevice(cfg["DEVICE_PATH"])
    except Exception as exc:
        print(f"discord-ptt: startup failed: {exc}", file=sys.stderr)
        return 1

    shortcut = str(cfg["DISCORD_SHORTCUT"])
    display = str(cfg.get("DISPLAY", ":0"))
    ptt_code = int(cfg["PTT_CODE"])
    is_pressed = False

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
