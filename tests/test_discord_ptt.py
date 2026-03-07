import importlib.util
import json
import sys
import tempfile
import types
import unittest
from pathlib import Path
from unittest import mock


REPO_ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = REPO_ROOT / "discord-ptt.py"


def load_module():
    fake_evdev = types.ModuleType("evdev")
    fake_evdev.InputDevice = object
    fake_evdev.list_devices = lambda: []
    fake_evdev.ecodes = types.SimpleNamespace(
        EV_KEY=1,
        BTN_276=276,
        BTN_EXTRA=275,
    )

    spec = importlib.util.spec_from_file_location("discord_ptt", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)

    original_evdev = sys.modules.get("evdev")
    sys.modules["evdev"] = fake_evdev
    try:
        spec.loader.exec_module(module)
    finally:
        if original_evdev is None:
            del sys.modules["evdev"]
        else:
            sys.modules["evdev"] = original_evdev

    return module


class DiscordPttTests(unittest.TestCase):
    def test_load_config_prefers_runtime_file_and_normalizes_values(self):
        module = load_module()

        with tempfile.TemporaryDirectory() as tmpdir:
            tmp_path = Path(tmpdir)
            static_path = tmp_path / "config.json"
            runtime_path = tmp_path / "config_detected.json"

            static_path.write_text(
                json.dumps(
                    {
                        "DEVICE_PATH": "/dev/input/event1",
                        "PTT_CODE": 275,
                        "DISCORD_SHORTCUT": "ctrl+shift+p",
                    }
                ),
                encoding="utf-8",
            )
            runtime_path.write_text(
                json.dumps(
                    {
                        "DEVICE_NAME": "Test Mouse",
                        "DEVICE_PATH": "/dev/input/event5",
                        "PTT_KEY": "BTN_276",
                        "DISCORD_SHORTCUT": " plus ",
                    }
                ),
                encoding="utf-8",
            )

            module.CONFIG_PATHS = (str(runtime_path), str(static_path))
            cfg = module.load_config()

        self.assertEqual(cfg["DEVICE_NAME"], "Test Mouse")
        self.assertEqual(cfg["DEVICE_PATH"], "/dev/input/event5")
        self.assertEqual(cfg["PTT_KEY"], 276)
        self.assertEqual(cfg["DISCORD_SHORTCUT"], "shift+equal")
        self.assertEqual(cfg["DISPLAY"], ":0")

    def test_resolve_ptt_code_accepts_named_button(self):
        module = load_module()

        self.assertEqual(module.resolve_ptt_code({"PTT_KEY": "BTN_276"}), 276)

    def test_send_uses_display_and_keydown_command(self):
        module = load_module()
        module.XDOTOOL_BIN = "/usr/bin/xdotool"

        with mock.patch.object(module.subprocess, "run") as run_mock:
            module.send("shift+equal", ":9", True)

        run_mock.assert_called_once()
        args, kwargs = run_mock.call_args
        self.assertEqual(args[0], ["/usr/bin/xdotool", "keydown", "shift+equal"])
        self.assertEqual(kwargs["env"]["DISPLAY"], ":9")
        self.assertFalse(kwargs["check"])


if __name__ == "__main__":
    unittest.main()
