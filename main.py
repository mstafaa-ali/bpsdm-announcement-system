import os
import sys
import threading
import time
import webbrowser
from core.paths import init_directories
from core.config import get_system_settings
from core.database import init_db
from core.media_helper import init_default_media
from core.autostart import set_autostart
from core.scheduler_service import scheduler_service
from core.tray import SystemTrayManager
from core.logger import logger, log_activity
from web.server import create_app

import socket

def find_available_port(starting_port: int) -> int:
    """Checks if starting_port is open. If occupied, iterates to find next available port."""
    port = starting_port
    while port < starting_port + 50:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            try:
                s.bind(("0.0.0.0", port))
                return port
            except OSError:
                port += 1
    return starting_port

def open_browser_delayed(port: int):
    time.sleep(1.2)
    url = f"http://127.0.0.1:{port}"
    logger.info(f"[Main] Auto-opening dashboard browser at {url}...")
    try:
        webbrowser.open(url)
    except Exception as e:
        logger.warning(f"[Main] Failed to auto-open browser: {e}")

def on_app_exit():
    logger.info("[Main] Shutting down application...")
    try:
        scheduler_service.scheduler.shutdown(wait=False)
    except Exception:
        pass
    os._exit(0)

def main():
    logger.info("=" * 60)
    logger.info("  Smart Office Announcement & Prayer Alert System v2.0.0")
    logger.info("=" * 60)

    # 1. Initialize persistent runtime storage and default files
    init_directories()
    init_db()
    init_default_media()

    settings = get_system_settings()
    configured_port = settings.get("port", 5050)
    port = find_available_port(configured_port)
    if port != configured_port:
        logger.warning(f"[Main] Port {configured_port} is busy. Switched to available port {port}.")

    auto_open = settings.get("auto_open_browser", True)
    autostart_boot = settings.get("auto_start_on_boot", True)

    # 2. Configure Windows startup if applicable
    if autostart_boot:
        set_autostart(True)

    # 3. Start background scheduler
    scheduler_service.start()

    # 4. Start system tray icon (background thread)
    tray = SystemTrayManager(on_exit=on_app_exit)
    tray.start_in_background()

    # 5. Delayed auto-open browser
    if auto_open:
        threading.Thread(target=open_browser_delayed, args=(port,), daemon=True).start()

    # 6. Start Web Server
    app = create_app()
    log_activity("system", "main", status="success", message=f"Application started on port {port}")
    
    try:
        # Run Flask server (threaded for concurrent dashboard requests)
        app.run(host="0.0.0.0", port=port, debug=False, use_reloader=False)
    except (KeyboardInterrupt, SystemExit):
        on_app_exit()

if __name__ == "__main__":
    main()
