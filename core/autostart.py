import sys
import os
from pathlib import Path
from core.logger import logger

REG_KEY = r"Software\Microsoft\Windows\CurrentVersion\Run"
APP_NAME = "SmartOfficeAnnouncer"

def set_autostart(enable: bool = True) -> bool:
    """
    Configures Windows registry auto-start on boot.
    Gracefully no-ops on non-Windows platforms.
    """
    if sys.platform != "win32":
        logger.info(f"[AutoStart] Platform is {sys.platform}. Windows Registry auto-start skipped.")
        return True

    try:
        import winreg
        exe_path = sys.executable if getattr(sys, 'frozen', False) else str(Path(__file__).resolve().parent.parent / "main.py")
        
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, REG_KEY, 0, winreg.KEY_SET_VALUE) as key:
            if enable:
                winreg.SetValueEx(key, APP_NAME, 0, winreg.REG_SZ, f'"{exe_path}"')
                logger.info(f"[AutoStart] Enabled auto-start on boot for {APP_NAME}.")
            else:
                try:
                    winreg.DeleteValue(key, APP_NAME)
                    logger.info(f"[AutoStart] Disabled auto-start on boot for {APP_NAME}.")
                except FileNotFoundError:
                    pass
        return True
    except Exception as e:
        logger.error(f"[AutoStart] Failed to configure Windows autostart: {e}")
        return False
