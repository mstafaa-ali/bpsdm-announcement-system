# 📢 Smart Office Announcement & Prayer Alert System (Go Engine)

Aplikasi pemutar pengumuman kantor dan pengingat waktu sholat otomatis berbasis desktop portabel (`.exe`) mandiri (*zero-dependency* di sisi klien) dengan arsitektur **Go Engine** yang sangat ringan, hemat memori RAM, dan optimal untuk PC spesifikasi terbatas (PC kentang).

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
     - *Secondary (Offline Fallback):* Kalkulasi astronomi MABIMS / Kemenag menggunakan algoritma koordinat hisab GPS bawaan (murni Go, tanpa library eksternal).
   - Waktu pengaman (*Ihtiyat* +2 menit default).
   - Fitur pengingat pra-adzan (T-2 menit sebelum waktu sholat).

3. **Audio Engine Berkecepatan Tinggi**
   - Mendukung format `.mp3` dan `.wav`.
   - Decoder audio internal murni Go (`go-mp3` & RIFF parser, tanpa CGO).
   - **Audio Device Enumeration**: Memilih sound card / port audio output spesifik yang terhubung ke PA Central Amplifier.
   - Penanganan tabrakan jadwal (*FIFO Queue*) untuk mencegah audio tumpang tindih.
   - Tombol **Emergency Broadcast** (sirene darurat pada volume maksimal) dan tombol **Stop Audio**.

4. **Penyimpanan Data Andal & Tahan Crash**
   - **Hybrid Storage**:
     - `config.json` untuk konfigurasi sistem utama.
     - **SQLite (`data.db`)** dengan mode **WAL (Write-Ahead Logging)** menggunakan pure-Go driver (`modernc.org/sqlite`) yang tahan terhadap mati listrik mendadak.

5. **Desktop & System Integration**
   - **Auto-Start on Boot**: Registrasi otomatis ke Windows Registry (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`).
   - **System Tray Icon**: Berjalan di latar belakang dengan menu buka dashboard & keluar.
   - **Desktop Toast Notification**: Notifikasi visual Windows Toast saat pengumuman diputar.
   - **Backup & Restore**: Ekspor dan impor konfigurasi lengkap via file `.json`.

---

## 📁 Struktur Direktori

```
📁 announcer/
├── SmartOfficeAnnouncer.exe   ← Binary utama (hasil build Go)
├── config.json                ← Pengaturan port, volume, timezone, chime
├── data.db                    ← SQLite: jadwal statis, prayer config, activity logs
├── 📁 internal/               ← Modul backend Go (audio, database, prayer, scheduler, server, tray)
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
├── 📁 templates/              ← Web Dashboard UI (index.html)
├── 📁 logs/                   ← Log aktivitas sistem
├── build_go_windows.bat       ← Script build Go di Windows
├── build_cross_windows.sh     ← Script cross-compile Windows .exe dari Mac/Linux
└── run_app.bat                ← Shortcut untuk menjalankan aplikasi di Windows
```

---

## 🚀 Cara Menjalankan dalam Mode Pengembangan (Development)

1. Pastikan **Go 1.22+** sudah terpasang di komputer:
   ```bash
   go version
   ```

2. Jalankan aplikasi langsung dari terminal:
   ```bash
   go run main.go
   ```

3. Buka browser di:
   ```
   http://127.0.0.1:5050
   ```

---

## 📦 Cara Membangun Executable Windows (.exe)

### Opsi A: Membangun di Komputer Windows
Cukup jalankan file:
```cmd
build_go_windows.bat
```
Script akan mengompilasi binary Windows mandiri (tanpa dependensi CGO) ke dalam folder `Release_App_Go\`.

### Opsi B: Cross-Compile dari Mac / Linux
Jalankan script bash:
```bash
./build_cross_windows.sh
```
Binary Windows `SmartOfficeAnnouncer.exe` akan langsung terbuat di folder `Release_App_Go\`.
Folder `Release_App_Go` bersifat portabel dan dapat langsung disalin ke PC kantor.
