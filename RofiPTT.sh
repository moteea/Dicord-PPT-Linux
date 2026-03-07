#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="$HOME/.config/ptt"
CONFIG_PATH="$CONFIG_DIR/config.json"
RUNTIME_CONFIG_PATH="$CONFIG_DIR/config_detected.json"
DETECTOR_PATH="$CONFIG_DIR/DeviceDetector.py"
SERVICE="discord-ptt.service"
THEME_PATH="$(cd "$(dirname "$0")" && pwd)/ptt.rasi"

if [[ -f "$THEME_PATH" ]]; then
  ROFI_CMD=(rofi -theme "$THEME_PATH")
else
  ROFI_CMD=(rofi)
fi

notify() {
  if command -v notify-send >/dev/null 2>&1; then
    notify-send "Discord PTT" "$1"
  else
    printf "%s\n" "$1"
  fi
}

config_file() {
  if [[ -f "$RUNTIME_CONFIG_PATH" ]]; then
    printf "%s" "$RUNTIME_CONFIG_PATH"
  else
    printf "%s" "$CONFIG_PATH"
  fi
}

require_config() {
  local target
  target="$(config_file)"
  if [[ ! -f "$target" ]]; then
    notify "Missing config: $target"
    exit 1
  fi
}

normalize_shortcut() {
  local shortcut="$1"
  shortcut="$(echo "$shortcut" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  case "$shortcut" in
    ""|"+"|"plus")
      shortcut="shift+equal"
      ;;
  esac
  printf "%s" "$shortcut"
}

pick_shortcut() {
  local picked shortcut
  picked="$(printf "shift+equal\nCustom..." | "${ROFI_CMD[@]}" -dmenu -i -p "Discord keybind")"

  if [[ "$picked" == "Custom..." ]]; then
    shortcut="$("${ROFI_CMD[@]}" -dmenu -p "Type keybind" -mesg "Example: ctrl+shift+p or f8")"
  else
    shortcut="$picked"
  fi

  shortcut="$(normalize_shortcut "$shortcut")"
  if [[ ! "$shortcut" =~ ^[a-z0-9_+:-]+$ ]]; then
    notify "Invalid keybind format: $shortcut"
    exit 1
  fi

  printf "%s" "$shortcut"
}

restart_service_if_needed() {
  if systemctl --user is-enabled "$SERVICE" >/dev/null 2>&1 || systemctl --user is-active "$SERVICE" >/dev/null 2>&1; then
    systemctl --user restart "$SERVICE" || true
  fi
}

set_shortcut() {
  local target shortcut
  require_config
  target="$(config_file)"
  shortcut="$(pick_shortcut)"

  python3 - "$target" "$shortcut" <<'PY'
import json
import os
import sys

path, shortcut = sys.argv[1], sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    data = json.load(handle)
data["DISCORD_SHORTCUT"] = shortcut
tmp_path = f"{path}.tmp"
with open(tmp_path, "w", encoding="utf-8") as handle:
    json.dump(data, handle, indent=2)
    handle.write("\n")
os.replace(tmp_path, path)
PY

  restart_service_if_needed
  notify "Saved keybind: $shortcut"
}

show_shortcut() {
  local target current
  require_config
  target="$(config_file)"
  current="$(python3 - "$target" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
print(data.get("DISCORD_SHORTCUT", "not-set"))
PY
)"
  notify "Current keybind: $current"
}

launch_detector() {
  local shortcut="$1"
  local command

  if [[ ! -x "$DETECTOR_PATH" ]]; then
    notify "Missing detector: $DETECTOR_PATH"
    exit 1
  fi

  command="DISCORD_SHORTCUT='$shortcut'"
  if [[ -n "${PTT_NIX_CONFIG_PATH:-}" ]]; then
    command="$command PTT_NIX_CONFIG_PATH='${PTT_NIX_CONFIG_PATH}'"
  fi
  command="$command python3 '$DETECTOR_PATH'"

  if command -v kitty >/dev/null 2>&1; then
    kitty --title "Discord PTT Setup" --hold sh -lc "$command"
  elif command -v gnome-terminal >/dev/null 2>&1; then
    gnome-terminal -- sh -lc "$command; printf '\nPress Enter to close...'; read -r _"
  elif command -v xterm >/dev/null 2>&1; then
    xterm -hold -e sh -lc "$command"
  else
    sh -lc "$command"
  fi

  restart_service_if_needed
}

setup_device_and_keybind() {
  local shortcut
  shortcut="$(pick_shortcut)"
  launch_detector "$shortcut"
}

status="$(systemctl --user is-active "$SERVICE" 2>/dev/null || true)"

if [[ "$status" == "active" ]]; then
  options="$(printf "Stop PTT Service\nRestart PTT Service\nSet Discord Keybind\nSetup Device & Keybind\nShow Current Keybind")"
else
  options="$(printf "Start PTT Service\nSet Discord Keybind\nSetup Device & Keybind\nShow Current Keybind")"
fi

choice="$(echo "$options" | "${ROFI_CMD[@]}" -dmenu -i -p "Discord PTT")"

case "$choice" in
  "Start PTT Service")
    systemctl --user start "$SERVICE"
    notify "PTT service started"
    ;;
  "Stop PTT Service")
    systemctl --user stop "$SERVICE"
    notify "PTT service stopped"
    ;;
  "Restart PTT Service")
    systemctl --user restart "$SERVICE"
    notify "PTT service restarted"
    ;;
  "Set Discord Keybind")
    set_shortcut
    ;;
  "Setup Device & Keybind")
    setup_device_and_keybind
    ;;
  "Show Current Keybind")
    show_shortcut
    ;;
  *)
    exit 0
    ;;
esac
