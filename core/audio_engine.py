import os
import queue
import threading
import time
import numpy as np
import sounddevice as sd
import soundfile as sf
from pathlib import Path
from typing import List, Dict, Any, Optional, Tuple
from core.paths import MEDIA_DIR
from core.config import get_system_settings
from core.logger import log_activity, logger

class AudioPlaybackTask:
    def __init__(self, file_path: Path, volume: Optional[float] = None, 
                 prepend_chime: bool = True, source: str = "manual", 
                 is_emergency: bool = False, target_device: Optional[str] = None,
                 on_complete=None):
        self.file_path = file_path
        self.volume = volume
        self.prepend_chime = prepend_chime
        self.source = source
        self.is_emergency = is_emergency
        self.target_device = target_device
        self.on_complete = on_complete

class AudioEngine:
    def __init__(self):
        self._queue = queue.Queue()
        self._is_running = True
        self._is_playing = False
        self._current_source = None
        self._worker_thread = threading.Thread(target=self._process_queue, daemon=True)
        self._worker_thread.start()
        logger.info("[AudioEngine] Initialized audio queue worker thread.")

    def get_output_devices(self) -> List[Dict[str, Any]]:
        """Returns a list of all available audio output devices."""
        devices = []
        try:
            device_list = sd.query_devices()
            for idx, dev in enumerate(device_list):
                if dev.get("max_output_channels", 0) > 0:
                    devices.append({
                        "id": idx,
                        "name": dev["name"],
                        "channels": dev["max_output_channels"],
                        "default_samplerate": dev["default_samplerate"],
                        "is_default": idx == sd.default.device[1]
                    })
        except Exception as e:
            logger.error(f"[AudioEngine] Error querying audio devices: {e}")
        return devices

    def check_device_health(self, target_device_name: Optional[str] = None) -> Tuple[bool, str]:
        """
        Checks if the configured audio device is available.
        Returns (is_healthy, status_message).
        """
        try:
            available_devices = self.get_output_devices()
            if not available_devices:
                return False, "Tidak ada audio output device yang terdeteksi pada sistem."
            
            if not target_device_name:
                return True, "Menggunakan default audio device OS (Sehat)."
            
            found = any(target_device_name.lower() in dev["name"].lower() for dev in available_devices)
            if found:
                return True, f"Audio device '{target_device_name}' terdeteksi dan aktif."
            else:
                return False, f"Audio device '{target_device_name}' tidak ditemukan! Menggunakan fallback."
        except Exception as e:
            return False, f"Error memeriksa audio device: {e}"

    def get_target_device_id(self, target_device_name: Optional[str] = None) -> Optional[int]:
        """Resolves device name to integer index for sounddevice."""
        if not target_device_name:
            return None  # sounddevice default
        try:
            available_devices = self.get_output_devices()
            for dev in available_devices:
                if target_device_name.lower() in dev["name"].lower():
                    return dev["id"]
        except Exception as e:
            logger.error(f"[AudioEngine] Error finding device id: {e}")
        return None

    def load_audio(self, file_path: Path) -> Tuple[np.ndarray, int]:
        """
        Loads audio file (.mp3, .wav, etc.) and returns (numpy_array, sample_rate).
        Attempts soundfile first, falls back to pydub if needed.
        """
        if not file_path.exists():
            raise FileNotFoundError(f"Audio file not found: {file_path}")
        
        try:
            # Try soundfile directly
            data, samplerate = sf.read(str(file_path), dtype="float32")
            return data, samplerate
        except Exception as sf_err:
            logger.debug(f"[AudioEngine] soundfile read failed ({sf_err}), trying pydub...")
            from pydub import AudioSegment
            seg = AudioSegment.from_file(str(file_path))
            samples = np.array(seg.get_array_of_samples())
            if seg.channels == 2:
                samples = samples.reshape((-1, 2))
            
            # Normalize to float32 between -1.0 and 1.0
            if seg.sample_width == 2:  # 16-bit
                data = samples.astype(np.float32) / 32768.0
            elif seg.sample_width == 4:  # 32-bit
                data = samples.astype(np.float32) / 2147483648.0
            else:
                data = samples.astype(np.float32) / (2 ** (seg.sample_width * 8 - 1))
            
            return data, seg.frame_rate

    def _process_queue(self):
        """Background FIFO worker loop that plays queued audio items sequentially."""
        while self._is_running:
            try:
                task: AudioPlaybackTask = self._queue.get(timeout=1.0)
            except queue.Empty:
                continue

            try:
                self._is_playing = True
                self._current_source = task.source
                self._execute_playback(task)
            except Exception as e:
                logger.error(f"[AudioEngine] Playback failed for {task.file_path.name}: {e}")
                log_activity(
                    event_type="error",
                    source=task.source,
                    audio_file=task.file_path.name,
                    status="failed",
                    message=str(e)
                )
            finally:
                self._is_playing = False
                self._current_source = None
                self._queue.task_done()
                if task.on_complete:
                    try:
                        task.on_complete()
                    except Exception:
                        pass

    def _execute_playback(self, task: AudioPlaybackTask):
        settings = get_system_settings()
        target_device_name = task.target_device if task.target_device is not None else settings.get("audio_device")
        device_id = self.get_target_device_id(target_device_name)
        
        # Calculate volume
        effective_vol = 1.0 if task.is_emergency else (task.volume if task.volume is not None else settings.get("volume", 0.85))
        effective_vol = max(0.0, min(1.0, float(effective_vol)))

        # 1. Play Chime if enabled
        if task.prepend_chime and not task.is_emergency:
            chime_name = settings.get("chime_file", "chime.mp3")
            chime_path = MEDIA_DIR / chime_name
            if chime_path.exists():
                try:
                    chime_data, chime_sr = self.load_audio(chime_path)
                    chime_data = chime_data * effective_vol
                    try:
                        sd.play(chime_data, samplerate=chime_sr, device=device_id)
                        sd.wait()
                    except Exception as dev_err:
                        logger.warning(f"[AudioEngine] Chime playback on device {device_id} failed ({dev_err}), falling back to default device.")
                        sd.play(chime_data, samplerate=chime_sr, device=None)
                        sd.wait()
                    time.sleep(0.3)  # Brief pause between chime and speech
                except Exception as chime_err:
                    logger.warning(f"[AudioEngine] Could not play chime {chime_name}: {chime_err}")

        # 2. Play Main Audio
        data, sr = self.load_audio(task.file_path)
        data = data * effective_vol
        
        logger.info(f"[AudioEngine] Playing '{task.file_path.name}' (Vol: {effective_vol*100:.0f}%, Device: {target_device_name or 'Default'})...")
        
        try:
            sd.play(data, samplerate=sr, device=device_id)
            sd.wait()
        except Exception as dev_err:
            logger.warning(f"[AudioEngine] Playback on device {device_id} failed ({dev_err}), falling back to default device.")
            sd.play(data, samplerate=sr, device=None)
            sd.wait()

        # Log success
        log_activity(
            event_type="playback",
            source=task.source,
            audio_file=task.file_path.name,
            audio_device=target_device_name or "Default OS",
            status="success",
            message=f"Playback completed (Vol: {int(effective_vol*100)}%)"
        )

    def play_scheduled(self, file_name: str, volume: Optional[float] = None, 
                       prepend_chime: bool = True, source: str = "schedule",
                       target_device: Optional[str] = None):
        """Queue a scheduled announcement."""
        file_path = MEDIA_DIR / file_name
        task = AudioPlaybackTask(
            file_path=file_path,
            volume=volume,
            prepend_chime=prepend_chime,
            source=source,
            is_emergency=False,
            target_device=target_device
        )
        self._queue.put(task)

    def play_emergency(self, file_name: str = "emergency.mp3"):
        """Trigger emergency broadcast (stops current if possible or pushes to queue with max volume)."""
        sd.stop()  # Stop any sound currently playing immediately
        file_path = MEDIA_DIR / file_name
        task = AudioPlaybackTask(
            file_path=file_path,
            volume=1.0,
            prepend_chime=False,
            source="emergency",
            is_emergency=True
        )
        self._queue.put(task)

    def is_busy(self) -> bool:
        return self._is_playing or not self._queue.empty()

    def get_status(self) -> Dict[str, Any]:
        settings = get_system_settings()
        dev_healthy, dev_msg = self.check_device_health(settings.get("audio_device"))
        return {
            "is_playing": self._is_playing,
            "current_source": self._current_source,
            "queue_size": self._queue.qsize(),
            "device_healthy": dev_healthy,
            "device_message": dev_msg,
            "active_device": settings.get("audio_device") or "Default System Device"
        }

audio_engine = AudioEngine()
