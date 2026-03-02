# Discord Push-To-Talk on Linux

Use this if you want button Push-To-Talk in Discord on Linux.

## 1) Open Discord in X11 mode (important on Wayland)
Run Discord with:

```bash
discord --enable-features=UseOzonePlatform --ozone-platform=x11
```

Use this Discord session to set your keybind.

## 2) Set Discord PTT keybind
In Discord:
1. `User Settings -> Voice & Video`
2. Set `Input Mode = Push to Talk`
3. Set PTT keybind to `Shift + =`

If you use another combo, it must match your script config later.

## 3) Install required packages
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

## 4) Find your mouse input device
```bash
ls -l /dev/input/by-id/
```

Find your mouse and note its `eventX` device.

## 5) Find your side-button key code
```bash
sudo evtest /dev/input/eventX
```

Press your side button and note the code (example: `BTN_276` / `276`).

## 6) Create config file
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

## 7) Create the PTT script
Create `~/.config/ptt/discord-ptt.py`:

```python
#!/usr/bin/env python3
from evdev import InputDevice, ecodes
import json
import os
import subprocess

cfg = json.load(open(os.path.expanduser("~/.config/ptt/config.json")))
dev = InputDevice(cfg["DEVICE_PATH"])

def send(press):
    os.environ["DISPLAY"] = cfg.get("DISPLAY", ":0")
    cmd = "keydown" if press else "keyup"
    subprocess.run(["xdotool", cmd, cfg["DISCORD_SHORTCUT"]], check=False)

for event in dev.read_loop():
    if event.type == ecodes.EV_KEY and event.code == int(cfg["PTT_CODE"]):
        if event.value == 1:
            send(True)
        elif event.value == 0:
            send(False)
```

Make it executable:

```bash
chmod +x ~/.config/ptt/discord-ptt.py
```

## 8) Auto-start at login (systemd user service)
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

## 9) Nix / Home Manager setup
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
- Replace `DEVICE_PATH` (`/dev/input/eventX`) with your real input device.
- Replace `PTT_CODE` with your mouse side-button code from `evtest`.

Apply Home Manager:

```bash
home-manager switch
```

This module installs dependencies, writes the PTT scripts, and enables the user service.

## 10) Optional Rofi menu (non-Nix users)
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

## 11) Troubleshooting
1. Confirm Discord keybind matches `DISCORD_SHORTCUT` in your config.
2. Check logs:

```bash
journalctl --user -u discord-ptt.service -f
```

3. Make sure Discord is running with:

```bash
discord --enable-features=UseOzonePlatform --ozone-platform=x11
```
