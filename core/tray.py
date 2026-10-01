import threading
import webbrowser
from PIL import Image, ImageDraw
from core.logger import logger
from core.config import get_system_settings

class SystemTrayManager:
    def __init__(self, on_exit=None):
        self.icon = None
        self.on_exit = on_exit
        self.thread = None

    def create_image(self):
        # Generate clean speaker/broadcast icon in PIL
        width = 64
        height = 64
        image = Image.new('RGBA', (width, height), (0, 0, 0, 0))
        dc = ImageDraw.Draw(image)
        # Background circle
        dc.ellipse((4, 4, 60, 60), fill="#2563eb")
        # Speaker body shape
        dc.polygon([(18, 24), (28, 24), (38, 14), (38, 50), (28, 40), (18, 40)], fill="white")
        # Sound waves
        dc.arc((36, 20, 52, 44), start=-60, end=60, fill="white", width=3)
        return image

    def open_dashboard(self, icon=None, item=None):
        settings = get_system_settings()
        port = settings.get("port", 5000)
        url = f"http://127.0.0.1:{port}"
        webbrowser.open(url)

    def exit_action(self, icon=None, item=None):
        if self.icon:
            self.icon.stop()
        if self.on_exit:
            self.on_exit()

    def run(self):
        try:
            import sys
            if sys.platform == "darwin":
                # macOS Cocoa requires NSApplication to run on main thread; skip on dev Mac
                logger.debug("[SystemTray] System tray skipped on macOS dev thread.")
                return
            import pystray
            image = self.create_image()
            menu = pystray.Menu(
                pystray.MenuItem("Buka Dashboard", self.open_dashboard, default=True),
                pystray.MenuItem("Keluar", self.exit_action)
            )
            self.icon = pystray.Icon("SmartOfficeAnnouncer", image, "Smart Office Announcer", menu)
            self.icon.run()
        except Exception as e:
            logger.debug(f"[SystemTray] System tray not supported in this environment: {e}")

    def start_in_background(self):
        self.thread = threading.Thread(target=self.run, daemon=True)
        self.thread.start()
