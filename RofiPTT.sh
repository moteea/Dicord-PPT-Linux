#!/usr/bin/env bash
set -euo pipefail

CONFIG="$HOME/.config/ptt/config.json"
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

require_config() {
  if [[ ! -f "$CONFIG" ]]; then
    notify "Missing config: $CONFIG"
    exit 1
  fi
}

status=$(systemctl --user is-active "$SERVICE" 2>/dev/null || true)

if [[ "$status" == "active" ]]; then
  options=$(printf "Stop PTT Service\nRestart PTT Service\nSet Discord Keybind\nShow Current Keybind")
else
  options=$(printf "Start PTT Service\nSet Discord Keybind\nShow Current Keybind")
fi

choice=$(echo "$options" | "${ROFI_CMD[@]}" -dmenu -i -p "Discord PTT")

normalize_shortcut() {
  local s="$1"
  s=$(echo "$s" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
  case "$s" in
    "+"|"plus") s="shift+equal" ;;
  esac
  printf "%s" "$s"
}

set_shortcut() {
  local picked custom shortcut
  picked=$(printf "shift+equal\nCustom..." | "${ROFI_CMD[@]}" -dmenu -i -p "Discord keybind")

  if [[ "$picked" == "Custom..." ]]; then
    custom=$("${ROFI_CMD[@]}" -dmenu -p "Type keybind" -mesg "Example: ctrl+shift+p or f8")
    shortcut=$(normalize_shortcut "$custom")
  else
    shortcut=$(normalize_shortcut "$picked")
  fi

  if [[ -z "$shortcut" ]]; then
    notify "No keybind entered"
    exit 0
  fi

  if [[ ! "$shortcut" =~ ^[a-z0-9_+:-]+$ ]]; then
    notify "Invalid keybind format: $shortcut"
    exit 1
  fi

  require_config
  python3 - "$CONFIG" "$shortcut" <<'PY'
import json
import os
import sys
p, key = sys.argv[1], sys.argv[2]
with open(p, 'r', encoding='utf-8') as f:
    data = json.load(f)
data['DISCORD_SHORTCUT'] = key
tmp = f"{p}.tmp"
with open(tmp, 'w', encoding='utf-8') as f:
    json.dump(data, f, separators=(',', ':'))
os.replace(tmp, p)
PY

  notify "Saved keybind: $shortcut"
  if systemctl --user is-enabled "$SERVICE" >/dev/null 2>&1; then
    systemctl --user restart "$SERVICE" || true
  fi
}

show_shortcut() {
  local current
  require_config
  current=$(python3 - "$CONFIG" <<'PY'
import json
import sys
with open(sys.argv[1], 'r', encoding='utf-8') as f:
    data = json.load(f)
print(data.get('DISCORD_SHORTCUT', 'not-set'))
PY
)
  notify "Current keybind: $current"
}

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
  "Show Current Keybind")
    show_shortcut
    ;;
  *)
    exit 0
    ;;
esac
