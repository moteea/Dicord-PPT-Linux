{ config, pkgs, lib, ... }:

let
  pttConfig = {
    DEVICE_NAME = "";
    DEVICE_PATH = "";
    PTT_KEY = "";
    PTT_CODE = 0;
    DISCORD_SHORTCUT = "shift+equal";
    DISPLAY = ":0";
  };

  goMod = ''
    module discord-ptt-go

    go 1.25.0

    require github.com/grafov/evdev v1.0.0
  '';

  goSum = ''
    github.com/grafov/evdev v1.0.0 h1:/rsOssITVhM7GPMyvlT34kiAOnvYzKJnKvuOqaMYW7c=
    github.com/grafov/evdev v1.0.0/go.mod h1:dcJfZnwr3VySHuphTV+Q0JcbP3AWR9W4WqbLeA3bG6U=
  '';

  goMain = builtins.readFile ./main.go;

  goSource = pkgs.runCommand "discord-ptt-go-src" {} ''
    mkdir -p "$out"
    cat > "$out/go.mod" <<'EOF'
    ${goMod}
    EOF
    cat > "$out/go.sum" <<'EOF'
    ${goSum}
    EOF
    cat > "$out/main.go" <<'EOF'
    ${goMain}
    EOF
  '';

  discordPttGo = pkgs.buildGoModule {
    pname = "discord-ptt-go";
    version = "0.1.0";
    src = goSource;

    vendorHash = "sha256-4HDlQjrCAgiblScGfYdeDsCGCH40jhwSggQZ3GlCyX8=";

    ldflags = [
      "-s"
      "-w"
    ];
  };
in
{
  home.packages = with pkgs; [
    discordPttGo
    rofi
    libnotify
    xdotool
  ];

  xdg.configFile."ptt-go/config.json".text = builtins.toJSON pttConfig;

  home.activation.ensurePTTGoRuntimeConfig = config.lib.dag.entryAfter [ "writeBoundary" ] ''
    $DRY_RUN_CMD mkdir -p "$HOME/.config/ptt-go"
    if [ ! -f "$HOME/.config/ptt-go/config_detected.json" ]; then
      $DRY_RUN_CMD cp -f "$HOME/.config/ptt-go/config.json" "$HOME/.config/ptt-go/config_detected.json"
    fi
    if [ ! -f "$HOME/.config/ptt-go/shortcut_override.json" ]; then
      cat > "$HOME/.config/ptt-go/shortcut_override.json" <<'EOF'
    {
      "DISCORD_SHORTCUT": "shift+equal"
    }
    EOF
    fi
  '';

  home.activation.installPTTGoManager = config.lib.dag.entryAfter [ "writeBoundary" ] ''
    $DRY_RUN_CMD mkdir -p "$HOME/.config/ptt-go"
    cat > "$HOME/.config/ptt-go/PTTManager.sh" <<'EOF'
    #!/usr/bin/env bash
    set -euo pipefail

    PTT_BIN="${discordPttGo}/bin/discord-ptt-go"
    CONFIG_DIR="$HOME/.config/ptt-go"
    PIDFILE="/tmp/discord-ptt-go.pid"
    LOGFILE="/tmp/discord-ptt-go.log"

    notify() {
      local message="$1"
      local urgency="''${2:-normal}"
      if command -v notify-send >/dev/null 2>&1; then
        notify-send -u "$urgency" "PTT Manager" "$message"
      else
        printf '%s\n' "$message"
      fi
    }

    get_pids() {
      pgrep -f "$PTT_BIN daemon" 2>/dev/null || true
    }

    is_running() {
      local pids
      pids="$(get_pids)"
      if [[ -n "$pids" ]]; then
        printf '%s\n' "$pids" | tail -n1 > "$PIDFILE"
        return 0
      fi
      rm -f "$PIDFILE"
      return 1
    }

    start_ptt() {
      if is_running; then
        notify "PTT service is already running"
        return 0
      fi

      nohup "$PTT_BIN" daemon --config-dir "$CONFIG_DIR" > "$LOGFILE" 2>&1 &
      sleep 1

      if is_running; then
        notify "PTT service started"
      else
        notify "Failed to start PTT service" critical
        return 1
      fi
    }

    stop_ptt() {
      if ! is_running; then
        notify "PTT service is not running"
        return 0
      fi

      local pids
      pids="$(get_pids)"
      for pid in $pids; do
        kill "$pid" 2>/dev/null || true
      done

      sleep 1
      rm -f "$PIDFILE"
      notify "PTT service stopped"
    }

    status_ptt() {
      if is_running; then
        printf 'running:%s\n' "$(cat "$PIDFILE")"
      else
        printf 'stopped\n'
      fi
    }

    setup_ptt() {
      if command -v kitty >/dev/null 2>&1; then
        kitty --title "PTT Setup" --hold sh -lc "'$PTT_BIN' setup --config-dir '$CONFIG_DIR'"
      else
        "$PTT_BIN" setup --config-dir "$CONFIG_DIR"
      fi
    }

    logs_ptt() {
      if [[ -f "$LOGFILE" ]]; then
        if command -v kitty >/dev/null 2>&1; then
          kitty --title "PTT Logs" --hold sh -lc "tail -f '$LOGFILE'"
        else
          notify "Logs: $LOGFILE"
        fi
      else
        notify "No log file found"
      fi
    }

    case "''${1:-help}" in
      start) start_ptt ;;
      stop) stop_ptt ;;
      restart) stop_ptt; sleep 1; start_ptt ;;
      status) status_ptt ;;
      setup) setup_ptt ;;
      logs) logs_ptt ;;
      *)
        echo "Usage: $0 {start|stop|restart|status|setup|logs}"
        ;;
    esac
    EOF
    chmod +x "$HOME/.config/ptt-go/PTTManager.sh"
  '';

  home.activation.installRofiPTTGo = config.lib.dag.entryAfter [ "writeBoundary" ] ''
    cat > "$HOME/.config/ptt-go/RofiPTT.sh" <<'EOF'
    #!/usr/bin/env bash
    set -euo pipefail

    PTT_MANAGER="$HOME/.config/ptt-go/PTTManager.sh"

    get_status() {
      "$PTT_MANAGER" status
    }

    create_menu() {
      local status
      status="$(get_status)"
      if [[ "$status" == stopped ]]; then
        printf '%s\n' \
          "Start PTT Service" \
          "Setup Device & Keybind" \
          "View Logs" \
          "Help"
      else
        printf '%s\n' \
          "Stop PTT Service" \
          "Restart PTT Service" \
          "Setup Device & Keybind" \
          "View Logs" \
          "Help"
      fi
    }

    choice="$(create_menu | rofi -dmenu -i -p 'PTT Go')"

    case "$choice" in
      "Start PTT Service") "$PTT_MANAGER" start ;;
      "Stop PTT Service") "$PTT_MANAGER" stop ;;
      "Restart PTT Service") "$PTT_MANAGER" restart ;;
      "Setup Device & Keybind") "$PTT_MANAGER" setup ;;
      "View Logs") "$PTT_MANAGER" logs ;;
      "Help")
        notify-send "PTT Manager" "Use setup to auto-detect the mouse button, then start the daemon."
        ;;
      *) exit 0 ;;
    esac
    EOF
    chmod +x "$HOME/.config/ptt-go/RofiPTT.sh"
  '';
}
