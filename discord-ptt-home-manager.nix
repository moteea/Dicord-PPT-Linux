{ config, pkgs, ... }:

let
  pttConfig = {
    DEVICE_PATH = "/dev/input/eventX";
    PTT_CODE = 276;
    DISCORD_SHORTCUT = "shift+equal";
    DISPLAY = ":0";
  };
in
{
  home.packages = with pkgs; [
    python3
    python3Packages.evdev
    xdotool
    rofi
    libnotify
  ];

  xdg.configFile."ptt/config.json".text = builtins.toJSON pttConfig;

  xdg.configFile."ptt/discord-ptt.py" = {
    executable = true;
    text = builtins.readFile ./discord-ptt.py;
  };

  xdg.configFile."ptt/RofiPTT.sh" = {
    executable = true;
    text = builtins.readFile ./RofiPTT.sh;
  };

  xdg.configFile."ptt/ptt.rasi" = {
    text = builtins.readFile ./ptt.rasi;
  };

  systemd.user.services.discord-ptt = {
    Unit = {
      Description = "Discord Mouse Push-To-Talk";
      After = [ "graphical-session.target" ];
    };

    Service = {
      ExecStart = "%h/.config/ptt/discord-ptt.py";
      Restart = "always";
      Environment = [ "DISPLAY=:0" ];
    };

    Install = {
      WantedBy = [ "default.target" ];
    };
  };
}
