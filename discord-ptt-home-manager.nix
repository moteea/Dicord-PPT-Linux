{ config, pkgs, ... }:

let
  pttPython = pkgs.python3.withPackages (ps: [ ps.evdev ]);
  pttConfig = {
    DEVICE_NAME = "Your Mouse Name";
    DEVICE_PATH = "/dev/input/eventX";
    PTT_KEY = "BTN_276";
    PTT_CODE = 276;
    DISCORD_SHORTCUT = "shift+equal";
    DISPLAY = ":0";
  };
in
{
  home.packages = with pkgs; [
    pttPython
    xdotool
    rofi
    libnotify
  ];

  xdg.configFile."ptt/config.json".text = builtins.toJSON pttConfig;

  xdg.configFile."ptt/discord-ptt.py" = {
    executable = true;
    text = builtins.readFile ./discord-ptt.py;
  };

  xdg.configFile."ptt/DeviceDetector.py" = {
    executable = true;
    text = builtins.readFile ./DeviceDetector.py;
  };

  xdg.configFile."ptt/RofiPTT.sh" = {
    executable = true;
    text = builtins.readFile ./RofiPTT.sh;
  };

  xdg.configFile."ptt/ptt.rasi".text = builtins.readFile ./ptt.rasi;

  home.activation.ensurePTTRuntimeConfig = config.lib.dag.entryAfter [ "writeBoundary" ] ''
    $DRY_RUN_CMD mkdir -p "$HOME/.config/ptt"
    if [ ! -f "$HOME/.config/ptt/config_detected.json" ]; then
      $DRY_RUN_CMD cp -f "$HOME/.config/ptt/config.json" "$HOME/.config/ptt/config_detected.json"
    fi
  '';

  systemd.user.services.discord-ptt = {
    Unit = {
      Description = "Discord Push-to-Talk";
      After = [ "graphical-session.target" ];
      PartOf = [ "graphical-session.target" ];
    };

    Service = {
      ExecStart = "${pttPython}/bin/python %h/.config/ptt/discord-ptt.py";
      Restart = "always";
      Environment = [
        "DISPLAY=:0"
        "XDOTOOL_BIN=${pkgs.xdotool}/bin/xdotool"
      ];
    };

    Install = {
      WantedBy = [ "default.target" ];
    };
  };
}
