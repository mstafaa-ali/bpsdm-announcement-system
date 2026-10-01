import json
import os
import shutil
import tempfile
from typing import Any, Dict
from core.paths import CONFIG_FILE

DEFAULT_CONFIG: Dict[str, Any] = {
    "system_settings": {
        "port": 5050,
        "timezone": "Asia/Jakarta",
        "volume": 0.85,
        "prepend_chime": True,
        "chime_file": "chime.mp3",
        "audio_device": None,
        "auto_open_browser": True,
        "auto_start_on_boot": True
    }
}

def load_config() -> Dict[str, Any]:
    """Load config.json, creating it with defaults if it does not exist."""
    if not CONFIG_FILE.exists():
        save_config(DEFAULT_CONFIG)
        return DEFAULT_CONFIG.copy()
    
    try:
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            data = json.load(f)
            # Ensure missing default keys are populated
            if "system_settings" not in data:
                data["system_settings"] = DEFAULT_CONFIG["system_settings"].copy()
            else:
                for k, v in DEFAULT_CONFIG["system_settings"].items():
                    if k not in data["system_settings"]:
                        data["system_settings"][k] = v
            return data
    except Exception as e:
        print(f"[Config] Error reading {CONFIG_FILE}: {e}. Returning defaults.")
        return DEFAULT_CONFIG.copy()

def save_config(config_data: Dict[str, Any]) -> bool:
    """Save config atomically using temporary file to prevent corruption on power loss."""
    try:
        dir_name = CONFIG_FILE.parent
        # Create temp file in the same directory for atomic replace across file systems
        with tempfile.NamedTemporaryFile("w", dir=dir_name, delete=False, encoding="utf-8") as tf:
            json.dump(config_data, tf, indent=2, ensure_ascii=False)
            temp_name = tf.name
        
        # Atomic rename
        shutil.move(temp_name, str(CONFIG_FILE))
        return True
    except Exception as e:
        print(f"[Config] Error saving config: {e}")
        return False

def get_system_settings() -> Dict[str, Any]:
    """Helper to get system_settings dictionary."""
    return load_config().get("system_settings", DEFAULT_CONFIG["system_settings"])

def update_system_settings(new_settings: Dict[str, Any]) -> Dict[str, Any]:
    """Update only provided fields in system_settings."""
    cfg = load_config()
    cfg["system_settings"].update(new_settings)
    save_config(cfg)
    return cfg["system_settings"]
