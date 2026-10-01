import numpy as np
import soundfile as sf
from pathlib import Path
from core.paths import MEDIA_DIR

def generate_tone(freq: float, duration: float, sr: int = 44100, fade_out: bool = True) -> np.ndarray:
    """Generates a clean sine wave tone."""
    t = np.linspace(0, duration, int(sr * duration), False)
    tone = np.sin(2 * np.pi * freq * t)
    if fade_out:
        envelope = np.linspace(1.0, 0.0, len(t)) ** 1.5
        tone = tone * envelope
    return tone.astype(np.float32)

def generate_chime_wave() -> np.ndarray:
    """Creates a 3-note melodic corporate chime (C5 -> E5 -> G5)."""
    sr = 44100
    n1 = generate_tone(523.25, 0.6, sr)  # C5
    n2 = generate_tone(659.25, 0.6, sr)  # E5
    n3 = generate_tone(783.99, 1.2, sr)  # G5
    silence = np.zeros(int(sr * 0.05), dtype=np.float32)
    chime = np.concatenate([n1, silence, n2, silence, n3])
    # Duplicate to stereo
    return np.column_stack([chime, chime])

def generate_emergency_siren() -> np.ndarray:
    """Creates a dual-tone emergency alert beep."""
    sr = 44100
    b1 = generate_tone(880.0, 0.4, sr, fade_out=False)
    b2 = generate_tone(587.33, 0.4, sr, fade_out=False)
    silence = np.zeros(int(sr * 0.1), dtype=np.float32)
    cycle = np.concatenate([b1, silence, b2, silence])
    siren = np.concatenate([cycle, cycle, cycle])
    return np.column_stack([siren, siren])

def generate_voice_cue(frequency: float, label: str) -> np.ndarray:
    """Generates pleasant audible notification tones."""
    sr = 44100
    n1 = generate_tone(frequency, 0.5, sr)
    n2 = generate_tone(frequency * 1.25, 0.8, sr)
    cue = np.concatenate([n1, n2])
    return np.column_stack([cue, cue])

def init_default_media():
    """Generates starter sound files if media directory is empty."""
    MEDIA_DIR.mkdir(parents=True, exist_ok=True)
    sr = 44100

    defaults = {
        "chime.mp3": generate_chime_wave(),
        "pre_adzan.mp3": generate_voice_cue(440.0, "pre_adzan"),
        "adzan_dzuhur.mp3": generate_voice_cue(523.25, "adzan_dzuhur"),
        "adzan_ashar.mp3": generate_voice_cue(587.33, "adzan_ashar"),
        "adzan_maghrib.mp3": generate_voice_cue(659.25, "adzan_maghrib"),
        "masuk_kantor.mp3": generate_voice_cue(493.88, "masuk_kantor"),
        "istirahat.mp3": generate_voice_cue(392.00, "istirahat"),
        "pulang_kantor.mp3": generate_voice_cue(349.23, "pulang_kantor"),
        "emergency.mp3": generate_emergency_siren(),
    }

    for filename, audio_data in defaults.items():
        out_path = MEDIA_DIR / filename
        if not out_path.exists():
            try:
                # Save as WAV first or MP3
                sf.write(str(out_path), audio_data, sr)
            except Exception:
                # If mp3 encoding not in libsndfile directly, save as wav or let soundfile handle
                wav_path = MEDIA_DIR / (Path(filename).stem + ".wav")
                if not wav_path.exists():
                    sf.write(str(wav_path), audio_data, sr)
