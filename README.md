# 📢 Smart Office Announcement & Prayer Alert System

Aplikasi pemutar pengumuman kantor dan pengingat waktu sholat otomatis berbasis desktop portabel (`.exe`) mandiri (*zero-dependency* di sisi klien) dengan dashboard web lokal yang modern dan responsif.

---

## ✨ Fitur Utama

1. **Jadwal Pengumuman Statis Otomatis**
   - Penjadwalan rutin (jam masuk, istirahat siang, jam pulang, dll.).
   - Pemilihan hari aktif (Senin–Jumat / custom).
   - Filter hari libur resmi nasional (`skip_holidays`).
   - Kontrol volume per-jadwal dan *prepend chime* (nada pembuka 2 detik).

2. **Pengingat Waktu Sholat Dinamis (Dzuhur, Ashar, Maghrib)**
   - Sinkronisasi harian otomatis pukul `00:01:00`.
   - **Multi-Source Resilience**:
     - *Primary:* REST API Kemenag / MyQuran.
     - *Secondary (Offline Fallback):* Kalkulasi astronomi MABIMS / Kemenag menggunakan koordinat GPS via `adhanpy`.
   - Waktu pengaman (*Ihtiyat* +2 menit default).
   - Fitur peringatan awal (*Pre-Alert T-Minus*, misal 10 menit sebelum masuk waktu sholat).

3. **Audio Engine & Device Selection**
   - Mendukung format `.mp3`, `.wav`, `.ogg`, `.flac`.
   - **Audio Device Enumeration**: Memilih sound card / port audio output spesifik yang terhubung ke PA Central Amplifier.
   - Penanganan tabrakan jadwal (*FIFO Queue*) untuk mencegah audio tumpang tindih.
   - Tombol **Emergency Broadcast** (sirene darurat pada volume maksimal).

4. **Penyimpanan Data Andal & Tahan Crash**
   - **Hybrid Storage**:
     - `config.json` (Atomic Write) untuk pengaturan sistem dasar.
     - **SQLite (`data.db`)** dengan mode **WAL (Write-Ahead Logging)** untuk menjamin konsistensi data saat mati listrik mendadak.

5. **Desktop & System Integration**
   - **Auto-Start on Boot**: Otomatis aktif saat Windows menyala / restart.
   - **System Tray Icon**: Berjalan di background dengan menu buka dashboard & keluar.
   - **Desktop Toast Notification**: Notifikasi visual saat pengumuman diputar.
   - **Backup & Restore**: Ekspor dan impor seluruh konfigurasi ke file `.json`.

---

## 📁 Struktur Direktori

```
📁 announcer/
├── announcer.exe              ← Binary utama (PyInstaller --onefile)
├── config.json                ← Pengaturan port, volume, timezone, chime
├── data.db                    ← SQLite: jadwal statis, prayer config, logs
├── 📁 media/                  ← File audio pengumuman & adzan (.mp3, .wav)
│   ├── chime.mp3
│   ├── adzan_dzuhur.mp3
│   ├── adzan_ashar.mp3
│   ├── adzan_maghrib.mp3
│   ├── pre_adzan.mp3
│   ├── masuk_kantor.mp3
│   ├── istirahat.mp3
│   ├── pulang_kantor.mp3
│   └── emergency.mp3
└── 📁 logs/                   ← Rotating log files (maks 5MB)
    └── announcer.log
```

---

## 🚀 Cara Menjalankan dalam Mode Pengembangan (Development)

1. Pastikan Python 3.11+ terpasang:
   ```bash
   python3 -m venv venv
   source venv/bin/activate  # atau venv\Scripts\activate di Windows
   pip install -r requirements.txt
   ```

2. Jalankan aplikasi:
   ```bash
   python main.py
   ```

3. Buka browser di:
   ```
   http://127.0.0.1:5000
   ```

---

## 📦 Cara Membangun Executable Windows (.exe)

Jalankan perintah PyInstaller menggunakan file `.spec` yang sudah dikonfigurasi:

```bash
pyinstaller build_windows.spec
```

Hasil file `.exe` mandiri akan berada di dalam folder `dist/SmartOfficeAnnouncer.exe`.
Letakkan file tersebut bersama folder `media/` untuk mendistribusikan ke PC kantor.
