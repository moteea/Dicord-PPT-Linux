# Discord Push-to-Talk (PTT) for Linux

Discord Push-to-Talk for Linux is a Python script that enables global PTT for Discord on Linux, including Wayland setups where Discord push-to-talk may not work reliably.

Keywords: Discord PTT Linux, Discord Push-to-Talk Linux, Discord Wayland Push-to-Talk, global push-to-talk for Discord on Linux.

Discord on Linux can have trouble with global Push-to-Talk, especially on Wayland. This project works around that by listening for an input event and sending the configured Discord shortcut.

Use this project if you want global Push-to-Talk for Discord on Linux using a mouse side button or another input device.

## What this project does
- Watches a Linux input device such as a mouse side button.
- Sends your configured Discord Push-to-Talk shortcut when that button is pressed and released.
- Runs as a user service so Discord push-to-talk is available after login.

## Who this project is for
- Linux users who want Discord Push-to-Talk to work with a mouse button or another input device.
- Wayland users who need a workaround for unreliable Discord global PTT behavior.
- X11 users who want a lightweight global push-to-talk setup for Discord on Linux.

## Supported environments
- Linux
- Wayland
- X11

## Quick start
1. Run Discord in X11 mode so you can set the Discord Push-to-Talk keybind.
2. Install the required packages for your Linux distribution.
3. Find your input device and button code.
4. Save the config file and run the included Python script as a user service.

## Installation
### 1) Open Discord in X11 mode (important on Wayland)
Run Discord with:

```bash
discord --enable-features=UseOzonePlatform --ozone-platform=x11
```

Use this Discord session to set your keybind.

### 2) Set Discord PTT keybind
In Discord:
1. `User Settings -> Voice & Video`
2. Set `Input Mode = Push to Talk`
3. Set PTT keybind to `Shift + =`

If you use another combo, it must match your script config later.

### 3) Install required packages
Core (required for PTT):

### Ubuntu / Debian
```bash
sudo apt update
sudo apt install python3 python3-pip xdotool evtest
pip3 install --user evdev
```

### Fedora
```bash
sudo dnf install python3 python3-pip xdotool evtest
pip3 install --user evdev
```

### Arch
```bash
sudo pacman -S python python-pip xdotool evtest
pip install --user evdev
```

Optional (only for `RofiPTT.sh` menu and notifications):

- Ubuntu / Debian: `sudo apt install rofi libnotify-bin`
- Fedora: `sudo dnf install rofi libnotify`
- Arch: `sudo pacman -S rofi libnotify`

## Configuration
### 4) Find your mouse input device
```bash
ls -l /dev/input/by-id/
```

Use your mouse device path from `/dev/input/by-id/` if available.

### 5) Find your side-button key code
```bash
sudo evtest /dev/input/eventX
```

Press your side button and note the code (example: `BTN_276` / `276`).

### 6) Create config file
Create `~/.config/ptt/config.json`:

```json
{
  "DEVICE_PATH": "/dev/input/eventX",
  "PTT_CODE": 276,
  "DISCORD_SHORTCUT": "shift+equal",
  "DISPLAY": ":0"
}
```

Replace with your actual values.

### 7) Create the PTT script
Create `~/.config/ptt/discord-ptt.py`:

```python
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
```

Make it executable:

```bash
chmod +x ~/.config/ptt/discord-ptt.py
```

### 8) Auto-start at login (systemd user service)
Create `~/.config/systemd/user/discord-ptt.service`:

```ini
[Unit]
Description=Discord Mouse PTT

[Service]
ExecStart=%h/.config/ptt/discord-ptt.py
Restart=always
Environment=DISPLAY=:0

[Install]
WantedBy=default.target
```

Enable it:

```bash
systemctl --user daemon-reload
systemctl --user enable --now discord-ptt.service
```

## Alternative setup: Nix / Home Manager
This repo includes a Home Manager module:

`./discord-ptt-home-manager.nix`

Add it to your Home Manager config, for example:

```nix
{
  imports = [
    ./discord-ptt-home-manager.nix
  ];
}
```

Then edit your generated config values in:

- `~/.config/ptt/config.json`

Important:
- Replace `DEVICE_PATH` with your real input device. A `/dev/input/by-id/*event-mouse` path is preferred.
- Replace `PTT_CODE` with your mouse side-button code from `evtest`.

Apply Home Manager:

```bash
home-manager switch
```

This module installs dependencies, writes the PTT scripts, and enables the user service.

## Optional Rofi menu
This repo includes `RofiPTT.sh`.

Copy and run it:

```bash
mkdir -p ~/.config/ptt
cp ./RofiPTT.sh ~/.config/ptt/RofiPTT.sh
chmod +x ~/.config/ptt/RofiPTT.sh
~/.config/ptt/RofiPTT.sh
```

What it does:
- Start/stop/restart `discord-ptt.service`
- Set Discord keybind (preset or custom)
- Show current saved keybind

## Troubleshooting
1. Confirm Discord keybind matches `DISCORD_SHORTCUT` in your config.
2. Check logs:

```bash
journalctl --user -u discord-ptt.service -f
```

3. Make sure Discord is running with:

```bash
discord --enable-features=UseOzonePlatform --ozone-platform=x11
```

4. If the service starts but does not react to button presses, confirm your user can read the chosen input device.

## License
MIT. See [LICENSE](./LICENSE).
