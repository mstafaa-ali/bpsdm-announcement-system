import sqlite3
import json
import datetime
from typing import List, Dict, Any, Optional
from core.paths import DB_FILE

def get_db_connection() -> sqlite3.Connection:
    """Returns a SQLite connection configured with WAL mode and Row factory."""
    conn = sqlite3.connect(str(DB_FILE), timeout=10.0)
    conn.row_factory = sqlite3.Row
    # Enable WAL mode for high concurrency and crash resistance
    conn.execute("PRAGMA journal_mode=WAL;")
    conn.execute("PRAGMA synchronous=NORMAL;")
    return conn

def init_db():
    """Initializes tables and seeds initial data if needed."""
    with get_db_connection() as conn:
        cursor = conn.cursor()
        
        # 1. Static Schedules
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS static_schedules (
                id TEXT PRIMARY KEY,
                name TEXT NOT NULL,
                time TEXT NOT NULL,
                days TEXT NOT NULL,
                audio_file TEXT NOT NULL,
                volume REAL DEFAULT NULL,
                skip_holidays INTEGER DEFAULT 1,
                enabled INTEGER DEFAULT 1,
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL
            );
        """)
        
        # 2. Prayer Configuration (key-value store)
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS prayer_configuration (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            );
        """)
        
        # 3. Holidays
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS holidays (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                date TEXT NOT NULL UNIQUE,
                name TEXT
            );
        """)
        
        # 4. Activity Log
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS activity_log (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                timestamp TEXT NOT NULL,
                event_type TEXT NOT NULL,
                source TEXT,
                audio_file TEXT,
                audio_device TEXT,
                status TEXT NOT NULL,
                message TEXT
            );
        """)
        
        # Seed default prayer configuration if empty
        cursor.execute("SELECT COUNT(*) as count FROM prayer_configuration")
        if cursor.fetchone()["count"] == 0:
            default_prayer_configs = {
                "enabled": True,
                "city_id": "1301",
                "city_name": "Kota Jakarta",
                "latitude": -6.2088,
                "longitude": 106.8456,
                "ihtiyat_minutes": 2,
                "pre_alert_enabled": True,
                "pre_alert_offset_minutes": 10,
                "pre_alert_audio_file": "pre_adzan.mp3",
                "dzuhur_enabled": True,
                "dzuhur_audio_file": "adzan_dzuhur.mp3",
                "dzuhur_volume": None,
                "ashar_enabled": True,
                "ashar_audio_file": "adzan_ashar.mp3",
                "ashar_volume": None,
                "maghrib_enabled": True,
                "maghrib_audio_file": "adzan_maghrib.mp3",
                "maghrib_volume": None,
                "skip_days": ["saturday", "sunday"]
            }
            for k, v in default_prayer_configs.items():
                cursor.execute(
                    "INSERT INTO prayer_configuration (key, value) VALUES (?, ?)",
                    (k, json.dumps(v))
                )
        
        # Seed default sample schedules if empty
        cursor.execute("SELECT COUNT(*) as count FROM static_schedules")
        if cursor.fetchone()["count"] == 0:
            now = datetime.datetime.now().isoformat()
            sample_schedules = [
                (
                    "sch-01",
                    "Jam Masuk Kantor",
                    "07:30",
                    json.dumps(["monday", "tuesday", "wednesday", "thursday", "friday"]),
                    "masuk_kantor.mp3",
                    None,
                    1,
                    1,
                    now,
                    now
                ),
                (
                    "sch-02",
                    "Istirahat Siang",
                    "12:00",
                    json.dumps(["monday", "tuesday", "wednesday", "thursday"]),
                    "istirahat.mp3",
                    None,
                    1,
                    1,
                    now,
                    now
                ),
                (
                    "sch-03",
                    "Jam Pulang Kerja",
                    "16:30",
                    json.dumps(["monday", "tuesday", "wednesday", "thursday", "friday"]),
                    "pulang_kantor.mp3",
                    None,
                    1,
                    1,
                    now,
                    now
                )
            ]
            cursor.executemany("""
                INSERT INTO static_schedules (id, name, time, days, audio_file, volume, skip_holidays, enabled, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """, sample_schedules)
            
        conn.commit()

# --- Static Schedules CRUD ---

def get_all_schedules() -> List[Dict[str, Any]]:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM static_schedules ORDER BY time ASC")
        rows = cursor.fetchall()
        result = []
        for r in rows:
            d = dict(r)
            d["days"] = json.loads(d["days"])
            d["enabled"] = bool(d["enabled"])
            d["skip_holidays"] = bool(d["skip_holidays"])
            result.append(d)
        return result

def get_schedule_by_id(sch_id: str) -> Optional[Dict[str, Any]]:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM static_schedules WHERE id = ?", (sch_id,))
        row = cursor.fetchone()
        if not row:
            return None
        d = dict(row)
        d["days"] = json.loads(d["days"])
        d["enabled"] = bool(d["enabled"])
        d["skip_holidays"] = bool(d["skip_holidays"])
        return d

def create_schedule(data: Dict[str, Any]) -> Dict[str, Any]:
    now = datetime.datetime.now().isoformat()
    sch_id = data.get("id") or f"sch-{int(datetime.datetime.now().timestamp()*1000)}"
    days_json = json.dumps(data.get("days", ["monday", "tuesday", "wednesday", "thursday", "friday"]))
    
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            INSERT INTO static_schedules (id, name, time, days, audio_file, volume, skip_holidays, enabled, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            sch_id,
            data["name"],
            data["time"],
            days_json,
            data["audio_file"],
            data.get("volume"),
            1 if data.get("skip_holidays", True) else 0,
            1 if data.get("enabled", True) else 0,
            now,
            now
        ))
        conn.commit()
    return get_schedule_by_id(sch_id)

def update_schedule(sch_id: str, data: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    now = datetime.datetime.now().isoformat()
    existing = get_schedule_by_id(sch_id)
    if not existing:
        return None
    
    name = data.get("name", existing["name"])
    time_val = data.get("time", existing["time"])
    days_json = json.dumps(data.get("days", existing["days"]))
    audio_file = data.get("audio_file", existing["audio_file"])
    volume = data.get("volume", existing["volume"])
    skip_holidays = 1 if data.get("skip_holidays", existing["skip_holidays"]) else 0
    enabled = 1 if data.get("enabled", existing["enabled"]) else 0
    
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            UPDATE static_schedules
            SET name = ?, time = ?, days = ?, audio_file = ?, volume = ?, skip_holidays = ?, enabled = ?, updated_at = ?
            WHERE id = ?
        """, (name, time_val, days_json, audio_file, volume, skip_holidays, enabled, now, sch_id))
        conn.commit()
    return get_schedule_by_id(sch_id)

def toggle_schedule(sch_id: str) -> Optional[Dict[str, Any]]:
    existing = get_schedule_by_id(sch_id)
    if not existing:
        return None
    new_state = not existing["enabled"]
    return update_schedule(sch_id, {"enabled": new_state})

def delete_schedule(sch_id: str) -> bool:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("DELETE FROM static_schedules WHERE id = ?", (sch_id,))
        conn.commit()
        return cursor.rowcount > 0

# --- Prayer Configuration ---

def get_prayer_config() -> Dict[str, Any]:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT key, value FROM prayer_configuration")
        rows = cursor.fetchall()
        result = {}
        for r in rows:
            try:
                result[r["key"]] = json.loads(r["value"])
            except Exception:
                result[r["key"]] = r["value"]
        return result

def update_prayer_config(updates: Dict[str, Any]) -> Dict[str, Any]:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        for k, v in updates.items():
            val_str = json.dumps(v)
            cursor.execute("""
                INSERT INTO prayer_configuration (key, value) VALUES (?, ?)
                ON CONFLICT(key) DO UPDATE SET value = excluded.value
            """, (k, val_str))
        conn.commit()
    return get_prayer_config()

# --- Holidays ---

def get_holidays() -> List[Dict[str, Any]]:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM holidays ORDER BY date ASC")
        return [dict(r) for r in cursor.fetchall()]

def is_holiday(date_str: str) -> bool:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT COUNT(*) as cnt FROM holidays WHERE date = ?", (date_str,))
        return cursor.fetchone()["cnt"] > 0

def add_holiday(date_str: str, name: str = "") -> bool:
    try:
        with get_db_connection() as conn:
            cursor = conn.cursor()
            cursor.execute("INSERT OR REPLACE INTO holidays (date, name) VALUES (?, ?)", (date_str, name))
            conn.commit()
            return True
    except Exception:
        return False

def delete_holiday(holiday_id: int) -> bool:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("DELETE FROM holidays WHERE id = ?", (holiday_id,))
        conn.commit()
        return cursor.rowcount > 0

# --- Activity Log ---

def insert_activity_log(event_type: str, source: str, audio_file: Optional[str] = None,
                        audio_device: Optional[str] = None, status: str = "success",
                        message: Optional[str] = None) -> int:
    now = datetime.datetime.now().isoformat()
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            INSERT INTO activity_log (timestamp, event_type, source, audio_file, audio_device, status, message)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        """, (now, event_type, source, audio_file, audio_device, status, message))
        conn.commit()
        return cursor.lastrowid

def get_recent_activity_logs(limit: int = 50) -> List[Dict[str, Any]]:
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM activity_log ORDER BY id DESC LIMIT ?", (limit,))
        return [dict(r) for r in cursor.fetchall()]
