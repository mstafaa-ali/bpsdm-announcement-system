import datetime
import pytz
from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger
from typing import Dict, Any, List, Optional

from core.database import (
    get_all_schedules, get_prayer_config, is_holiday,
    get_recent_activity_logs, insert_activity_log
)
from core.config import get_system_settings
from core.audio_engine import audio_engine
from core.prayer_resolver import prayer_resolver
from core.notifications import show_notification
from core.logger import logger, log_activity

DAY_MAPPING = {
    "monday": "mon",
    "tuesday": "tue",
    "wednesday": "wed",
    "thursday": "thu",
    "friday": "fri",
    "saturday": "sat",
    "sunday": "sun"
}

class SchedulerService:
    def __init__(self):
        job_defaults = {
            'coalesce': True,
            'misfire_grace_time': 60  # Tolerate up to 60 seconds delay after wake up
        }
        self.scheduler = BackgroundScheduler(job_defaults=job_defaults)
        self._today_prayer_data: Optional[Dict[str, Any]] = None
        self._next_job_info: Optional[Dict[str, Any]] = None

    def start(self):
        """Starts the background scheduler and loads initial jobs."""
        if not self.scheduler.running:
            self.scheduler.start()
            logger.info("[Scheduler] APScheduler engine started.")
        self.reload_jobs()

    def reload_jobs(self):
        """Clears existing announcement jobs and reloads them from DB without restart."""
        logger.info("[Scheduler] Reloading all scheduler jobs...")
        self.scheduler.remove_all_jobs()

        settings = get_system_settings()
        tz_name = settings.get("timezone", "Asia/Jakarta")
        try:
            tz = pytz.timezone(tz_name)
        except Exception:
            tz = pytz.timezone("Asia/Jakarta")

        # 1. Register midnight prayer sync job (runs everyday at 00:01:00)
        self.scheduler.add_job(
            self._midnight_sync_task,
            trigger=CronTrigger(hour=0, minute=1, second=0, timezone=tz),
            id="midnight_prayer_sync",
            replace_existing=True
        )

        # 2. Register periodic audio device health check (every 5 mins)
        self.scheduler.add_job(
            self._periodic_health_check,
            trigger=IntervalTrigger(minutes=5, timezone=tz),
            id="device_health_check",
            replace_existing=True
        )

        # 3. Register static schedules from DB
        schedules = get_all_schedules()
        for sch in schedules:
            if not sch.get("enabled", True):
                continue
            
            time_parts = sch["time"].split(":")
            if len(time_parts) != 2:
                continue
            hour, minute = int(time_parts[0]), int(time_parts[1])
            
            # Map days
            days_cron = ",".join([DAY_MAPPING.get(d.lower(), d[:3].lower()) for d in sch.get("days", []) if d.lower() in DAY_MAPPING])
            if not days_cron:
                days_cron = "mon-fri"

            self.scheduler.add_job(
                self._execute_static_schedule,
                trigger=CronTrigger(day_of_week=days_cron, hour=hour, minute=minute, timezone=tz),
                args=[sch],
                id=f"static_{sch['id']}",
                replace_existing=True,
                name=sch["name"]
            )

        # 4. Resolve and register today's prayer times
        self._sync_and_schedule_prayers(tz)
        logger.info(f"[Scheduler] Successfully registered {len(self.scheduler.get_jobs())} jobs.")

    def _periodic_health_check(self):
        """Periodic device and system health verification."""
        settings = get_system_settings()
        target_dev = settings.get("audio_device")
        healthy, msg = audio_engine.check_device_health(target_dev)
        if not healthy:
            logger.warning(f"[Scheduler Health Check] {msg}")

    def _midnight_sync_task(self):
        """Triggered at 00:01 daily to calculate prayer times and schedule jobs for the new day."""
        settings = get_system_settings()
        tz = pytz.timezone(settings.get("timezone", "Asia/Jakarta"))
        self._sync_and_schedule_prayers(tz)

    def _sync_and_schedule_prayers(self, tz):
        """Fetches/calculates today's prayer times and registers dynamic jobs."""
        now = datetime.datetime.now(tz)
        today_date = now.date()
        today_weekday = today_date.strftime("%A").lower()

        prayer_data = prayer_resolver.get_today_schedule(today_date)
        self._today_prayer_data = prayer_data

        if not prayer_data.get("enabled", False):
            return

        cfg = prayer_data.get("config", {})
        skip_days = [d.lower() for d in cfg.get("skip_days", ["saturday", "sunday"])]
        if today_weekday in skip_days:
            logger.info(f"[Scheduler] Skipping prayer times today ({today_weekday} is in skip_days).")
            return

        # Register jobs ONLY for prayer reminder (T-2 mins before prayer)
        times = prayer_data.get("times", {})
        reminders = prayer_data.get("reminders", {})

        for prayer_name in ["dzuhur", "ashar", "maghrib"]:
            is_enabled = cfg.get(f"{prayer_name}_enabled", True)
            if not is_enabled:
                continue

            # Reminder Job (default 2 minutes before prayer)
            reminder_time_str = reminders.get(prayer_name)
            if reminder_time_str and ":" in reminder_time_str:
                h, m = map(int, reminder_time_str.split(":"))
                audio_file = cfg.get(f"{prayer_name}_audio_file", "pre_adzan.mp3")
                vol = cfg.get(f"{prayer_name}_volume")
                
                self.scheduler.add_job(
                    self._execute_prayer_reminder,
                    trigger=CronTrigger(hour=h, minute=m, timezone=tz),
                    args=[prayer_name, audio_file, vol],
                    id=f"prayer_reminder_{prayer_name}",
                    replace_existing=True,
                    name=f"Pengingat Menjelang Sholat {prayer_name.capitalize()}"
                )

    def _execute_static_schedule(self, sch: Dict[str, Any]):
        """Callback for static schedule triggers."""
        today_str = datetime.date.today().isoformat()
        
        # Check holiday filter
        if sch.get("skip_holidays", True) and is_holiday(today_str):
            logger.info(f"[Scheduler] Skipping '{sch['name']}' because today ({today_str}) is marked as holiday.")
            log_activity(
                event_type="playback",
                source=sch["name"],
                audio_file=sch["audio_file"],
                status="skipped",
                message="Dilewati (Hari Libur)"
            )
            return

        settings = get_system_settings()
        prepend_chime = settings.get("prepend_chime", True)
        
        show_notification(
            title="Pengumuman Otomatis",
            message=f"Memutar pengumuman: {sch['name']}"
        )
        
        audio_engine.play_scheduled(
            file_name=sch["audio_file"],
            volume=sch.get("volume"),
            prepend_chime=prepend_chime,
            source=sch["name"]
        )

    def _execute_prayer_reminder(self, prayer_name: str, audio_file: str, volume: Optional[float]):
        """Callback for prayer reminder triggers (2 mins before prayer)."""
        title = f"Pengingat Menjelang Waktu Sholat {prayer_name.capitalize()}"
        
        show_notification(
            title="Pengingat Sholat",
            message=f"2 Menit Menjelang Waktu Sholat {prayer_name.capitalize()}"
        )
        
        settings = get_system_settings()
        prepend_chime = settings.get("prepend_chime", True)
        
        audio_engine.play_scheduled(
            file_name=audio_file,
            volume=volume,
            prepend_chime=prepend_chime,
            source=title
        )

    def get_next_run(self) -> Optional[Dict[str, Any]]:
        """Returns details about the next upcoming scheduled job."""
        jobs = self.scheduler.get_jobs()
        upcoming = []
        for j in jobs:
            if j.id in ["midnight_prayer_sync", "device_health_check"]:
                continue
            nxt = j.next_run_time
            if nxt:
                upcoming.append({
                    "id": j.id,
                    "name": j.name or j.id,
                    "next_run_time": nxt.isoformat(),
                    "timestamp": nxt.timestamp()
                })
        if not upcoming:
            return None
        upcoming.sort(key=lambda x: x["timestamp"])
        return upcoming[0]

    def get_status_overview(self) -> Dict[str, Any]:
        """Provides high-level system status for dashboard."""
        settings = get_system_settings()
        engine_status = audio_engine.get_status()
        next_run = self.get_next_run()
        tz_name = settings.get("timezone", "Asia/Jakarta")
        
        try:
            tz = pytz.timezone(tz_name)
            current_time = datetime.datetime.now(tz).strftime("%Y-%m-%d %H:%M:%S")
        except Exception:
            current_time = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        return {
            "current_time": current_time,
            "timezone": tz_name,
            "engine": engine_status,
            "next_run": next_run,
            "active_jobs_count": len(self.scheduler.get_jobs()),
            "today_prayers": self._today_prayer_data or prayer_resolver.get_today_schedule()
        }

scheduler_service = SchedulerService()
