#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="discord-ptt.service"
CONFIG_DIR="$HOME/.config/ptt"
CONFIG_FILE="$CONFIG_DIR/config.json"
PTT_SCRIPT="$CONFIG_DIR/discord-ptt.py"
SERVICE_DIR="$HOME/.config/systemd/user"
SERVICE_FILE="$SERVICE_DIR/$SERVICE_NAME"
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_PTT_SCRIPT="$REPO_DIR/discord-ptt.py"

info() { printf "[INFO] %s\n" "$*"; }
warn() { printf "[WARN] %s\n" "$*" >&2; }
err() { printf "[ERROR] %s\n" "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "Missing command: $1"
}

detect_pkg_manager() {
  if command -v apt >/dev/null 2>&1; then
    echo "apt"
  elif command -v dnf >/dev/null 2>&1; then
    echo "dnf"
  elif command -v pacman >/dev/null 2>&1; then
    echo "pacman"
  else
    echo "unknown"
  fi
}

install_packages() {
  local mgr="$1"
  local install_rofi="$2"

  case "$mgr" in
    apt)
      info "Installing core packages with apt..."
      sudo apt update
      sudo apt install -y python3 python3-pip xdotool evtest
      if [[ "$install_rofi" == "yes" ]]; then
        info "Installing optional Rofi packages..."
        sudo apt install -y rofi libnotify-bin
      fi
      ;;
    dnf)
      info "Installing core packages with dnf..."
      sudo dnf install -y python3 python3-pip xdotool evtest
      if [[ "$install_rofi" == "yes" ]]; then
        info "Installing optional Rofi packages..."
        sudo dnf install -y rofi libnotify
      fi
      ;;
    pacman)
      info "Installing core packages with pacman..."
      sudo pacman -Sy --noconfirm python python-pip xdotool evtest
      if [[ "$install_rofi" == "yes" ]]; then
        info "Installing optional Rofi packages..."
        sudo pacman -Sy --noconfirm rofi libnotify
      fi
      ;;
    *)
      warn "Unsupported package manager. Install dependencies manually:"
      warn "python3/python + pip, xdotool, evtest, and python evdev."
      ;;
  esac
}

install_evdev() {
  if python3 -c "import evdev" >/dev/null 2>&1; then
    info "python evdev already available."
    return
  fi

  info "Installing python evdev with pip..."
  python3 -m pip install --user evdev
}

pick_device() {
  local entries=()
  local path label

  if compgen -G "/dev/input/by-id/*event-mouse" >/dev/null; then
    for path in /dev/input/by-id/*event-mouse; do
      entries+=("$path")
    done
  fi

  if [[ ${#entries[@]} -eq 0 ]]; then
    warn "No /dev/input/by-id/*event-mouse found. Falling back to /dev/input/event*."
    if compgen -G "/dev/input/event*" >/dev/null; then
      for path in /dev/input/event*; do
        entries+=("$path")
      done
    fi
  fi

  [[ ${#entries[@]} -gt 0 ]] || err "No input devices found under /dev/input."

  info "Select your mouse input device:"
  local i=1
  for path in "${entries[@]}"; do
    label="$path"
    if [[ -L "$path" ]]; then
      label="$path -> $(readlink -f "$path")"
    fi
    printf "  %d) %s\n" "$i" "$label"
    i=$((i + 1))
  done

  local choice
  while true; do
    read -r -p "Enter number [1-${#entries[@]}]: " choice
    if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= ${#entries[@]} )); then
      break
    fi
    warn "Invalid selection."
  done

  printf "%s\n" "${entries[$((choice - 1))]}"
}

detect_ptt_code() {
  local device="$1"
  local out
  local resolved_device

  resolved_device="$(readlink -f "$device" 2>/dev/null || printf "%s" "$device")"

  info "Press your mouse side button once (5s timeout)..."
  out="$(sudo timeout 5s evtest "$resolved_device" 2>&1 || true)"

  local code
  code="$(printf "%s\n" "$out" | sed -nE 's/.*code[[:space:]]+([0-9]+)[[:space:]]+\(.+\),[[:space:]]+value[[:space:]]+1.*/\1/p' | head -n1)"

  if [[ -z "${code:-}" ]]; then
    warn "Auto-detection failed."
    warn "Tip: make sure you selected the right mouse device and press the side button quickly."
    read -r -p "Enter button code manually (example: 276): " code
  fi

  [[ "$code" =~ ^[0-9]+$ ]] || err "Invalid PTT code: $code"
  echo "$code"
}

write_config() {
  local device="$1"
  local code="$2"
  local shortcut="$3"
  local display="$4"

  mkdir -p "$CONFIG_DIR"

  cat > "$CONFIG_FILE" <<EOF
{
  "DEVICE_PATH": "$device",
  "PTT_CODE": $code,
  "DISCORD_SHORTCUT": "$shortcut",
  "DISPLAY": "$display"
}
EOF
}

write_ptt_script() {
  mkdir -p "$CONFIG_DIR"
  [[ -f "$REPO_PTT_SCRIPT" ]] || err "Missing repo script: $REPO_PTT_SCRIPT"
  install -m 755 "$REPO_PTT_SCRIPT" "$PTT_SCRIPT"
}

write_service() {
  mkdir -p "$SERVICE_DIR"
  cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Discord Mouse PTT

[Service]
ExecStart=%h/.config/ptt/discord-ptt.py
Restart=always
Environment=DISPLAY=$1

[Install]
WantedBy=default.target
EOF
}

enable_service() {
  systemctl --user daemon-reload
  systemctl --user enable --now "$SERVICE_NAME"
}

main() {
  require_cmd sudo
  require_cmd python3
  require_cmd systemctl

  local mgr install_rofi device code shortcut display_value
  mgr="$(detect_pkg_manager)"

  read -r -p "Install optional Rofi menu dependencies too? [y/N]: " install_rofi
  if [[ "${install_rofi,,}" == "y" || "${install_rofi,,}" == "yes" ]]; then
    install_rofi="yes"
  else
    install_rofi="no"
  fi

  install_packages "$mgr" "$install_rofi"
  install_evdev

  device="$(pick_device)"
  info "Selected device: $device"

  code="$(detect_ptt_code "$device")"
  info "Detected PTT code: $code"

  read -r -p "Discord shortcut to send [shift+equal]: " shortcut
  shortcut="${shortcut:-shift+equal}"
  shortcut="$(printf "%s" "$shortcut" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"

  display_value="${DISPLAY:-:0}"
  read -r -p "X11 DISPLAY [$display_value]: " display_input
  display_value="${display_input:-$display_value}"

  write_config "$device" "$code" "$shortcut" "$display_value"
  write_ptt_script
  write_service "$display_value"
  enable_service

  info "Setup complete."
  info "Config: $CONFIG_FILE"
  info "Service: $SERVICE_NAME (enabled and started)"
  info "Discord should be run in X11 mode:"
  info "discord --enable-features=UseOzonePlatform --ozone-platform=x11"
}

main "$@"
