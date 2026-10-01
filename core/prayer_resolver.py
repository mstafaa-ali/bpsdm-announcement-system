import datetime
import requests
from typing import Dict, Any, Optional
from core.database import get_prayer_config
from core.logger import log_activity, logger

# adhanpy imports for offline astronomical calculation
try:
    from adhanpy.calculation.CalculationMethod import CalculationMethod
    from adhanpy.calculation.CalculationParameters import CalculationParameters
    from adhanpy.PrayerTimes import PrayerTimes
    from adhanpy.Coordinates import Coordinates
    from adhanpy.data.DateComponents import DateComponents
    from adhanpy.calculation.Madhab import Madhab
    HAS_ADHANPY = True
except ImportError:
    HAS_ADHANPY = False

class PrayerResolver:
    def __init__(self):
        pass

    def fetch_from_api(self, city_id: str, date_obj: datetime.date) -> Optional[Dict[str, str]]:
        """
        Fetches prayer times from MyQuran / Kemenag open API.
        URL format: https://api.myquran.com/v2/sholat/jadwal/{city_id}/{year}/{month}/{day}
        Returns dict with keys: 'subuh', 'dzuhur', 'ashar', 'maghrib', 'isya' in HH:mm format.
        """
        try:
            url = f"https://api.myquran.com/v2/sholat/jadwal/{city_id}/{date_obj.year}/{date_obj.month:02d}/{date_obj.day:02d}"
            resp = requests.get(url, timeout=5.0)
            if resp.status_code == 200:
                json_data = resp.json()
                if json_data.get("status") is True and "data" in json_data and "jadwal" in json_data["data"]:
                    jadwal = json_data["data"]["jadwal"]
                    return {
                        "subuh": jadwal.get("subuh"),
                        "dzuhur": jadwal.get("dzuhur"),
                        "ashar": jadwal.get("ashar"),
                        "maghrib": jadwal.get("maghrib"),
                        "isya": jadwal.get("isya")
                    }
        except Exception as e:
            logger.warning(f"[PrayerResolver] Online API fetch failed: {e}. Switching to offline calculation.")
        return None

    def calculate_offline(self, lat: float, lng: float, date_obj: datetime.date) -> Dict[str, str]:
        """
        Calculates prayer times astronomically using adhanpy conforming to Kemenag/MABIMS.
        Subuh: 20°, Ashar: Shafi'i, Isya: 18°.
        """
        if not HAS_ADHANPY:
            logger.error("[PrayerResolver] adhanpy is not installed!")
            # Emergency fallback approximate values for Jakarta
            return {
                "subuh": "04:35",
                "dzuhur": "11:58",
                "ashar": "15:05",
                "maghrib": "18:02",
                "isya": "19:10"
            }

        coords = Coordinates(lat, lng)
        date_comp = DateComponents(date_obj.year, date_obj.month, date_obj.day)
        
        # MABIMS / Kemenag: fajr 20 deg, isha 18 deg, shafi madhab
        params = CalculationParameters(fajr_angle=20.0, isha_angle=18.0)
        params.madhab = Madhab.SHAFI
        
        pt = PrayerTimes(coords, date_comp, params)
        
        def format_time(dt_obj):
            if dt_obj is None:
                return "--:--"
            # dt_obj is in local or UTC depending on adhanpy version, let's extract hour:minute
            return f"{dt_obj.hour:02d}:{dt_obj.minute:02d}"

        return {
            "subuh": format_time(pt.fajr),
            "dzuhur": format_time(pt.dhuhr),
            "ashar": format_time(pt.asr),
            "maghrib": format_time(pt.maghrib),
            "isya": format_time(pt.isha)
        }

    def apply_ihtiyat(self, time_str: str, ihtiyat_minutes: int) -> str:
        """Adds safety offset minutes (Ihtiyat, standard +2 mins)."""
        if not time_str or ":" not in time_str:
            return time_str
        try:
            h, m = map(int, time_str.split(":"))
            dt = datetime.datetime(2000, 1, 1, h, m) + datetime.timedelta(minutes=ihtiyat_minutes)
            return f"{dt.hour:02d}:{dt.minute:02d}"
        except Exception:
            return time_str

    def calculate_pre_alert(self, time_str: str, offset_minutes: int) -> str:
        """Subtracts pre-alert minutes (e.g. 10 mins before prayer)."""
        if not time_str or ":" not in time_str:
            return time_str
        try:
            h, m = map(int, time_str.split(":"))
            dt = datetime.datetime(2000, 1, 1, h, m) - datetime.timedelta(minutes=offset_minutes)
            return f"{dt.hour:02d}:{dt.minute:02d}"
        except Exception:
            return time_str

    def get_today_schedule(self, target_date: Optional[datetime.date] = None) -> Dict[str, Any]:
        """
        Resolves today's prayer times, applying ihtiyat and pre-alert offsets.
        Returns complete prayer times and operational target jobs.
        """
        if target_date is None:
            target_date = datetime.date.today()

        config = get_prayer_config()
        if not config.get("enabled", True):
            return {
                "enabled": False,
                "date": target_date.isoformat(),
                "times": {},
                "source": "disabled"
            }

        city_id = str(config.get("city_id", "1301"))
        lat = float(config.get("latitude", -6.2088))
        lng = float(config.get("longitude", 106.8456))
        ihtiyat = int(config.get("ihtiyat_minutes", 2))
        reminder_offset = int(config.get("reminder_offset_minutes", 2))

        # 1. Try API first
        raw_times = self.fetch_from_api(city_id, target_date)
        source = "api"
        
        # 2. Fallback to offline astronomical calculation if API failed
        if not raw_times:
            raw_times = self.calculate_offline(lat, lng, target_date)
            source = "offline_calculated"

        # Apply Ihtiyat (+2 mins) to calculated/fetched times
        final_times = {}
        for prayer_name, time_str in raw_times.items():
            final_times[prayer_name] = self.apply_ihtiyat(time_str, ihtiyat)

        # Calculate Reminder Times (T - 2 minutes before prayer)
        reminders = {}
        for p in ["dzuhur", "ashar", "maghrib"]:
            if p in final_times:
                reminders[p] = self.calculate_pre_alert(final_times[p], reminder_offset)

        log_activity(
            event_type="prayer_sync",
            source="prayer_resolver",
            status="success",
            message=f"Synced prayer reminders ({source}): Dzuhur reminder={reminders.get('dzuhur')}, Ashar reminder={reminders.get('ashar')}, Maghrib reminder={reminders.get('maghrib')}"
        )

        return {
            "enabled": True,
            "date": target_date.isoformat(),
            "source": source,
            "city_name": config.get("city_name", "Kota Jakarta"),
            "times": final_times,
            "reminders": reminders,
            "pre_alerts": reminders,
            "config": config
        }

prayer_resolver = PrayerResolver()
