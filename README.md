# Discord Push-To-Talk on Linux

Use this if you want button Push-To-Talk in Discord on Linux.

## Quick setup script (automatic)
This repo includes a one-shot installer script that does:
- OS/package-manager detection
- dependency install
- mouse device selection
- side-button code detection
- config + service registration

Run:

```bash
chmod +x ./setup-ptt.sh
./setup-ptt.sh
```

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

Prefer the stable `/dev/input/by-id/*event-mouse` path instead of a raw `/dev/input/eventX` path.

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
Copy the repo script into place:

```bash
cp ./discord-ptt.py ~/.config/ptt/discord-ptt.py
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
- Replace `DEVICE_PATH` with your real input device. A `/dev/input/by-id/*event-mouse` path is preferred.
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

4. If the service starts but does not react to button presses, confirm your user can read the chosen input device.

## 12) License
MIT. See [LICENSE](./LICENSE).
