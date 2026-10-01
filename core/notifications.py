import sys
from typing import Optional
from core.logger import logger

def show_notification(title: str, message: str):
    """
    Shows a native desktop toast notification.
    Uses plyer / win10toast with fallback to console logging.
    """
    try:
        from plyer import notification
        notification.notify(
            title=title,
            message=message,
            app_name="Smart Office Announcer",
            timeout=5
        )
    except Exception as e:
        logger.debug(f"[Notification] Could not display toast notification: {e}")
