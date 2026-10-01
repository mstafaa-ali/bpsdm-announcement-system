import os
import shutil
import json
import datetime
from pathlib import Path
from flask import Flask, render_template, request, jsonify, send_from_directory, send_file, Response
from werkzeug.utils import secure_filename

from core.paths import MEDIA_DIR, CONFIG_FILE, DB_FILE, BUNDLE_DIR
from core.config import get_system_settings, update_system_settings, load_config, save_config
from core.database import (
    get_all_schedules, get_schedule_by_id, create_schedule, update_schedule,
    toggle_schedule, delete_schedule, get_prayer_config, update_prayer_config,
    get_recent_activity_logs, get_holidays, add_holiday, delete_holiday,
    get_db_connection
)
from core.audio_engine import audio_engine
from core.prayer_resolver import prayer_resolver
from core.scheduler_service import scheduler_service
from core.logger import log_activity, logger

ALLOWED_AUDIO_EXTENSIONS = {".mp3", ".wav", ".ogg", ".flac", ".m4a", ".aac"}

def create_app() -> Flask:
    template_folder = str(BUNDLE_DIR / "templates")
    static_folder = str(BUNDLE_DIR / "static")

    app = Flask(__name__, template_folder=template_folder, static_folder=static_folder)
    app.config["MAX_CONTENT_LENGTH"] = 50 * 1024 * 1024  # 50 MB max audio upload

    @app.route("/")
    def index():
        return render_template("index.html")

    # --- System & Status APIs ---

    @app.route("/api/status", methods=["GET"])
    def get_status():
        try:
            overview = scheduler_service.get_status_overview()
            return jsonify({"status": "success", "data": overview})
        except Exception as e:
            return jsonify({"status": "error", "message": str(e)}), 500

    @app.route("/api/devices", methods=["GET"])
    def get_devices():
        try:
            devices = audio_engine.get_output_devices()
            settings = get_system_settings()
            return jsonify({
                "status": "success",
                "devices": devices,
                "current_device": settings.get("audio_device")
            })
        except Exception as e:
            return jsonify({"status": "error", "message": str(e)}), 500

    @app.route("/api/settings", methods=["GET", "POST"])
    def handle_settings():
        if request.method == "GET":
            return jsonify({"status": "success", "settings": get_system_settings()})
        else:
            try:
                data = request.get_json() or {}
                updated = update_system_settings(data)
                # If autostart setting updated
                if "auto_start_on_boot" in data:
                    from core.autostart import set_autostart
                    set_autostart(bool(data["auto_start_on_boot"]))
                # Reload scheduler jobs with new timezone or settings
                scheduler_service.reload_jobs()
                return jsonify({"status": "success", "settings": updated})
            except Exception as e:
                return jsonify({"status": "error", "message": str(e)}), 500

    # --- Static Schedules CRUD ---

    @app.route("/api/schedules", methods=["GET", "POST"])
    def handle_schedules():
        if request.method == "GET":
            schedules = get_all_schedules()
            return jsonify({"status": "success", "schedules": schedules})
        else:
            try:
                data = request.get_json()
                if not data or "name" not in data or "time" not in data or "audio_file" not in data:
                    return jsonify({"status": "error", "message": "Nama, jam, dan file audio wajib diisi!"}), 400
                new_sch = create_schedule(data)
                scheduler_service.reload_jobs()
                log_activity("system", "web_dashboard", status="success", message=f"Created schedule '{data['name']}'")
                return jsonify({"status": "success", "schedule": new_sch})
            except Exception as e:
                return jsonify({"status": "error", "message": str(e)}), 500

    @app.route("/api/schedules/<sch_id>", methods=["GET", "PUT", "DELETE"])
    def handle_single_schedule(sch_id):
        if request.method == "GET":
            sch = get_schedule_by_id(sch_id)
            if not sch:
                return jsonify({"status": "error", "message": "Jadwal tidak ditemukan"}), 404
            return jsonify({"status": "success", "schedule": sch})
        elif request.method == "PUT":
            try:
                data = request.get_json() or {}
                updated = update_schedule(sch_id, data)
                if not updated:
                    return jsonify({"status": "error", "message": "Jadwal tidak ditemukan"}), 404
                scheduler_service.reload_jobs()
                log_activity("system", "web_dashboard", status="success", message=f"Updated schedule '{updated['name']}'")
                return jsonify({"status": "success", "schedule": updated})
            except Exception as e:
                return jsonify({"status": "error", "message": str(e)}), 500
        elif request.method == "DELETE":
            try:
                success = delete_schedule(sch_id)
                if success:
                    scheduler_service.reload_jobs()
                    log_activity("system", "web_dashboard", status="success", message=f"Deleted schedule {sch_id}")
                    return jsonify({"status": "success", "message": "Jadwal berhasil dihapus"})
                return jsonify({"status": "error", "message": "Jadwal tidak ditemukan"}), 404
            except Exception as e:
                return jsonify({"status": "error", "message": str(e)}), 500

    @app.route("/api/schedules/<sch_id>/toggle", methods=["POST"])
    def toggle_single_schedule(sch_id):
        try:
            updated = toggle_schedule(sch_id)
            if not updated:
                return jsonify({"status": "error", "message": "Jadwal tidak ditemukan"}), 404
            scheduler_service.reload_jobs()
            log_activity("system", "web_dashboard", status="success", message=f"Toggled schedule '{updated['name']}' to {'Active' if updated['enabled'] else 'Inactive'}")
            return jsonify({"status": "success", "schedule": updated})
        except Exception as e:
            return jsonify({"status": "error", "message": str(e)}), 500

    # --- Prayer Configuration APIs ---

    @app.route("/api/prayer", methods=["GET", "POST"])
    def handle_prayer_config():
        if request.method == "GET":
            cfg = get_prayer_config()
            return jsonify({"status": "success", "config": cfg})
        else:
            try:
                data = request.get_json() or {}
                updated = update_prayer_config(data)
                scheduler_service.reload_jobs()
                log_activity("system", "web_dashboard", status="success", message="Updated prayer configurations")
                return jsonify({"status": "success", "config": updated})
            except Exception as e:
                return jsonify({"status": "error", "message": str(e)}), 500

    @app.route("/api/prayer/today", methods=["GET"])
    def get_today_prayer():
        try:
            data = prayer_resolver.get_today_schedule()
            return jsonify({"status": "success", "data": data})
        except Exception as e:
            return jsonify({"status": "error", "message": str(e)}), 500

    # --- Media File Manager APIs ---

    @app.route("/api/media", methods=["GET"])
    def list_media():
        files = []
        if MEDIA_DIR.exists():
            for f in MEDIA_DIR.iterdir():
                if f.is_file() and f.suffix.lower() in ALLOWED_AUDIO_EXTENSIONS:
                    stat = f.stat()
                    files.append({
                        "filename": f.name,
                        "size_bytes": stat.st_size,
                        "size_mb": round(stat.st_size / (1024 * 1024), 2),
                        "modified": datetime.datetime.fromtimestamp(stat.st_mtime).isoformat()
                    })
        files.sort(key=lambda x: x["filename"].lower())
        return jsonify({"status": "success", "files": files})

    @app.route("/api/media/upload", methods=["POST"])
    def upload_media():
        if "file" not in request.files:
            return jsonify({"status": "error", "message": "Tidak ada file yang diunggah"}), 400
        
        file = request.files["file"]
        if file.filename == "":
            return jsonify({"status": "error", "message": "Nama file kosong"}), 400
        
        filename = secure_filename(file.filename)
        ext = Path(filename).suffix.lower()
        if ext not in ALLOWED_AUDIO_EXTENSIONS:
            return jsonify({"status": "error", "message": f"Format audio '{ext}' tidak didukung. Gunakan MP3, WAV, atau OGG."}), 400

        target_path = MEDIA_DIR / filename
        file.save(str(target_path))
        log_activity("system", "web_dashboard", audio_file=filename, status="success", message="Audio file uploaded")
        return jsonify({"status": "success", "filename": filename})

    @app.route("/api/media/<filename>", methods=["DELETE"])
    def delete_media(filename):
        target_path = MEDIA_DIR / secure_filename(filename)
        if target_path.exists() and target_path.is_file():
            target_path.unlink()
            log_activity("system", "web_dashboard", audio_file=filename, status="success", message="Audio file deleted")
            return jsonify({"status": "success", "message": "File audio berhasil dihapus"})
        return jsonify({"status": "error", "message": "File tidak ditemukan"}), 404

    @app.route("/api/media/stream/<filename>")
    def stream_media(filename):
        safe_name = secure_filename(filename)
        return send_from_directory(str(MEDIA_DIR), safe_name)

    # --- Live Audio Trigger / Test APIs ---

    @app.route("/api/audio/test", methods=["POST"])
    def test_audio():
        data = request.get_json() or {}
        file_name = data.get("file_name", "chime.mp3")
        volume = data.get("volume")
        prepend_chime = data.get("prepend_chime", False)
        target_device = data.get("device")
        
        target_file = MEDIA_DIR / file_name
        if not target_file.exists():
            return jsonify({"status": "error", "message": f"File {file_name} tidak ditemukan di folder media"}), 404
        
        audio_engine.play_scheduled(
            file_name=file_name,
            volume=volume,
            prepend_chime=prepend_chime,
            source="Manual Test",
            target_device=target_device
        )
        return jsonify({"status": "success", "message": f"Memutar '{file_name}'..."})

    @app.route("/api/audio/emergency", methods=["POST"])
    def trigger_emergency():
        data = request.get_json() or {}
        file_name = data.get("file_name", "emergency.mp3")
        audio_engine.play_emergency(file_name)
        log_activity("playback", "Emergency Broadcast", audio_file=file_name, status="success", message="EMERGENCY BROADCAST TRIGGERED!")
        return jsonify({"status": "success", "message": "Peringatan darurat (Emergency Broadcast) diaktifkan!"})

    @app.route("/api/audio/stop", methods=["POST"])
    def stop_audio():
        success = audio_engine.stop()
        if success:
            return jsonify({"status": "success", "message": "Pemutaran audio berhasil dihentikan."})
        return jsonify({"status": "error", "message": "Gagal menghentikan audio."}), 500

    # --- Activity Logs API ---

    @app.route("/api/logs", methods=["GET"])
    def get_logs():
        limit = int(request.args.get("limit", 50))
        logs = get_recent_activity_logs(limit)
        return jsonify({"status": "success", "logs": logs})

    # --- Backup & Restore Configuration ---

    @app.route("/api/backup/export", methods=["GET"])
    def export_backup():
        """Exports full JSON configuration including system settings, schedules, prayer configs, and holidays."""
        backup_data = {
            "version": "2.0.0",
            "exported_at": datetime.datetime.now().isoformat(),
            "system_settings": get_system_settings(),
            "prayer_configuration": get_prayer_config(),
            "static_schedules": get_all_schedules(),
            "holidays": get_holidays()
        }
        return Response(
            json.dumps(backup_data, indent=2, ensure_ascii=False),
            mimetype="application/json",
            headers={"Content-Disposition": f"attachment;filename=announcer_backup_{datetime.date.today().isoformat()}.json"}
        )

    @app.route("/api/backup/import", methods=["POST"])
    def import_backup():
        if "file" not in request.files:
            return jsonify({"status": "error", "message": "Tidak ada file backup yang diunggah"}), 400
        
        file = request.files["file"]
        try:
            content = json.load(file)
            # 1. Restore system settings
            if "system_settings" in content:
                update_system_settings(content["system_settings"])
            
            # 2. Restore prayer config
            if "prayer_configuration" in content:
                update_prayer_config(content["prayer_configuration"])
            
            # 3. Restore static schedules
            if "static_schedules" in content:
                with get_db_connection() as conn:
                    conn.execute("DELETE FROM static_schedules")
                for s in content["static_schedules"]:
                    create_schedule(s)
            
            # 4. Restore holidays
            if "holidays" in content:
                with get_db_connection() as conn:
                    conn.execute("DELETE FROM holidays")
                for h in content["holidays"]:
                    add_holiday(h["date"], h.get("name", ""))

            scheduler_service.reload_jobs()
            log_activity("system", "web_dashboard", status="success", message="Full configuration restored from backup")
            return jsonify({"status": "success", "message": "Konfigurasi berhasil dipulihkan dari file backup!"})
        except Exception as e:
            return jsonify({"status": "error", "message": f"Gagal membaca backup: {e}"}), 400

    return app
