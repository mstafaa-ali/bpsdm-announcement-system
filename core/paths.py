import os
import sys
from pathlib import Path

def get_app_dir() -> Path:
    """
    Returns the persistent directory where config, db, media, and logs should live.
    When frozen by PyInstaller, this is the directory containing the .exe file.
    When running in development, it's the project root.
    """
    if getattr(sys, 'frozen', False):
        return Path(sys.executable).resolve().parent
    return Path(__file__).resolve().parent.parent

def get_bundle_dir() -> Path:
    """
    Returns the bundle directory where internal read-only assets (HTML/CSS templates) are bundled.
    When frozen by PyInstaller, this is sys._MEIPASS.
    In development, it's the project root.
    """
    if getattr(sys, 'frozen', False) and hasattr(sys, '_MEIPASS'):
        return Path(sys._MEIPASS).resolve()
    return Path(__file__).resolve().parent.parent

APP_DIR = get_app_dir()
BUNDLE_DIR = get_bundle_dir()

MEDIA_DIR = APP_DIR / "media"
LOGS_DIR = APP_DIR / "logs"
CONFIG_FILE = APP_DIR / "config.json"
DB_FILE = APP_DIR / "data.db"

def init_directories():
    """Ensure all required runtime directories exist."""
    MEDIA_DIR.mkdir(parents=True, exist_ok=True)
    LOGS_DIR.mkdir(parents=True, exist_ok=True)
