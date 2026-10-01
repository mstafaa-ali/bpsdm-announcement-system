import logging
from logging.handlers import RotatingFileHandler
import os
from typing import Optional
from core.paths import LOGS_DIR, init_directories
from core.database import insert_activity_log

# Ensure logs dir exists
init_directories()

LOG_FILE = LOGS_DIR / "announcer.log"

logger = logging.getLogger("announcer")
logger.setLevel(logging.INFO)

if not logger.handlers:
    # 5MB per file, 3 backup files
    file_handler = RotatingFileHandler(
        str(LOG_FILE), maxBytes=5 * 1024 * 1024, backupCount=3, encoding="utf-8"
    )
    formatter = logging.Formatter(
        "[%(asctime)s] [%(levelname)s] [%(name)s]: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )
    file_handler.setFormatter(formatter)
    logger.addHandler(file_handler)

    # Console handler for development
    console_handler = logging.StreamHandler()
    console_handler.setFormatter(formatter)
    logger.addHandler(console_handler)

def log_activity(event_type: str, source: str, audio_file: Optional[str] = None,
                 audio_device: Optional[str] = None, status: str = "success",
                 message: Optional[str] = None):
    """
    Logs an event both to the rotating file logger and the SQLite activity_log table.
    """
    log_msg = f"[{event_type.upper()}] Source: {source} | File: {audio_file or 'N/A'} | Device: {audio_device or 'Default'} | Status: {status} | Msg: {message or 'OK'}"
    if status == "failed" or event_type == "error":
        logger.error(log_msg)
    else:
        logger.info(log_msg)
    
    try:
        insert_activity_log(
            event_type=event_type,
            source=source,
            audio_file=audio_file,
            audio_device=audio_device,
            status=status,
            message=message
        )
    except Exception as e:
        logger.error(f"Failed to record activity log in SQLite: {e}")
