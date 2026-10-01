# Product Requirement Document (PRD)

**Project Name:** Smart Office Announcement & Prayer Alert System

**Document Version:** 2.0.0

**Target Release:** Portable Desktop Binary (`.exe`)

**Target Environment:** Windows 10 / 11 (Headless / Mini-PC / Workstation)

---

## 1. Executive Summary & Objective

### 1.1 Problem Statement

Pengumuman operasional kantor (jam masuk, istirahat, jam pulang) serta pengingat waktu sholat saat ini dijalankan secara manual oleh staf kantor. Hal ini menyebabkan:

* Keterlambatan dan inkonsistensi waktu pengumuman.
* Kesulitan melacak waktu sholat (Dzuhur, Ashar & Maghrib) yang bergeser setiap hari secara astronomis.
* Keterikatan staf operasional pada jam-jam tertentu hanya untuk menekan tombol audio.

### 1.2 Objective

Membangun sistem pengumuman audio kantor otomatis berbasis perangkat lunak mandiri (*standalone portable application*) yang berjalan di latar belakang tanpa dependensi lingkungan eksternal (zero-dependency di sisi klien). Sistem menyediakan dashboard web lokal yang ringan untuk memudahkan admin non-teknis mengelola audio, mengatur jadwal statis, serta mengaktifkan pengingat sholat dinamis.

---

## 2. User Persona & Scope

* **Primary User (Admin/HR/General Affairs):** Mengakses dashboard lokal melalui browser web untuk mengunggah audio pengumuman, mengatur jam tayang, memilih lokasi kota, dan menguji volume speaker.
* **Secondary Beneficiary (Karyawan Kantor):** Mendengarkan pengumuman audio yang jernih, konsisten, dan tepat waktu melalui sistem tata suara (PA amplifier).

---

## 3. Functional Requirements (FR)

### FR-1: Static Announcement Scheduler

* **FR-1.1 (CRUD Jadwal):** Admin dapat membuat, membaca, memperbarui, dan menghapus jadwal pengumuman rutin.
* **FR-1.2 (Parameter Penjadwalan):** Setiap entitas jadwal mendukung:
  * Nama pengumuman.
  * Jam eksekusi (format 24 jam: `HH:mm`).
  * Hari aktif (Senin–Minggu; default: Senin–Jumat).
  * Pemilihan file audio target.
  * Volume per-jadwal (opsional; jika tidak diatur, menggunakan volume global).
  * Status aktif/nonaktif (*toggle switch*).
  * Opsi `skip_holidays` (melewati eksekusi pada hari libur).


* **FR-1.3 (Holiday Filter):** Pilihan untuk melewati eksekusi pada hari libur nasional resmi (dapat dikonfigurasi melalui daftar tanggal). *(Detail spesifikasi sumber data dan metode input akan ditentukan kemudian.)*

### FR-2: Dynamic Prayer Time Module

* **FR-2.1 (Penyelarasan Waktu Sholat Harian):** Sistem otomatis menghitung/mengambil jadwal sholat setiap hari pada pukul `00:01:00` untuk tanggal berjalan.
* **FR-2.2 (Multi-Source Resilience):**
  * *Primary:* Mengambil data waktu sholat harian dari REST API Kemenag / MyQuran berdasarkan ID Kota.
  * *Secondary (Offline Fallback):* Jika jaringan terputus, sistem otomatis menghitung waktu sholat secara offline menggunakan koordinat lintang & bujur lokasi kantor dengan metode perhitungan Kemenag RI / MABIMS (sudut Subuh 20°, Ashar Shafi'i, sudut Isya 18°).


* **FR-2.3 (Target Sholat Operasional):** Sistem mendukung 3 (tiga) waktu sholat operasional yang dapat diaktifkan/nonaktifkan secara individual:
  * **Dzuhur**
  * **Ashar**
  * **Maghrib**
* **FR-2.4 (Mode Pengingat Pra-Adzan T-2 Menit):** Sistem **hanya menyiarkan audio pengingat 2 menit sebelum masuk waktu sholat** (T-2 menit) agar jamaah dapat bersiap. Sistem **tidak memutar audio adzan saat waktu sholat tiba**.
* **FR-2.5 (Ihtiyat / Waktu Pengaman):** Penambahan offset waktu pengaman (+2 menit secara default sesuai standar hisab Kemenag).

### FR-3: Audio Playback Engine

* **FR-3.1 (Format Dukungan):** Mampu memutar file audio berekstensi `.mp3` dan `.wav`.
* **FR-3.2 (Chime Prepend):** Opsi otomatis untuk membunyikan nada pembuka (chime 2–3 detik) sebelum pesan audio utama diputar.
* **FR-3.3 (Concurrency & Overlap Handling):** Sistem memblokir pemutaran ganda jika dua jadwal bertabrakan secara bersamaan; pemutaran dilakukan secara sekuensial (*FIFO queue*).
* **FR-3.4 (Manual Trigger / Emergency Broadcast):** Tombol aksi langsung pada antarmuka untuk memutar audio tes speaker atau pengumuman prioritas/evakuasi sewaktu-waktu. Emergency broadcast selalu diputar pada volume maksimum.
* **FR-3.5 (Audio Device Selection):** Admin dapat memilih audio output device dari dashboard. Sistem menampilkan daftar audio output device yang terdeteksi oleh OS dan memungkinkan pemilihan device spesifik (bukan hanya default OS).
* **FR-3.6 (Audio Device Health Check):** Sistem secara periodik memeriksa ketersediaan audio output device yang dipilih. Jika device tidak terdeteksi (dicabut, berubah setelah restart), sistem menampilkan alert/warning di dashboard dan mencatat event ke log.
* **FR-3.7 (Audio Preview):** Admin dapat memutar preview file audio langsung dari browser dashboard sebelum file dijadwalkan, untuk memverifikasi konten dan kualitas audio.

### FR-4: Local Web UI Dashboard

* **FR-4.1 (Single-Port Access):** Dashboard disajikan via HTTP lokal (port default `5000`) yang dapat diakses di komputer lokal (`127.0.0.1`) maupun melalui LAN kantor (`http://<IP_KOMPUTER>:port`).
* **FR-4.2 (Audio File Manager):** Fitur upload file audio langsung dari browser yang otomatis tersimpan ke direktori media aplikasi.
* **FR-4.3 (Status Indicator):** Menampilkan status engine audio (Aktif/Siaga), audio device yang aktif, jam server terkini (beserta timezone), dan jadwal pengumuman berikutnya yang akan berbunyi (*Next Run Countdown*).
* **FR-4.4 (Zero-Reload Update):** Perubahan jadwal yang disimpan oleh admin langsung memicu pembaruan memori engine tanpa me-restart aplikasi.
* **FR-4.5 (Activity Log Viewer):** Dashboard menampilkan 50 entri log aktivitas terakhir (playback events, error, prayer time sync) secara real-time.
* **FR-4.6 (Backup & Restore):** Admin dapat mengekspor seluruh konfigurasi (jadwal, pengaturan sholat, system settings) sebagai file `.json` dan mengimpornya kembali untuk keperluan migrasi antar PC atau pemulihan konfigurasi.

### FR-5: Activity Log & Diagnostics

* **FR-5.1 (Playback Event Logging):** Sistem mencatat setiap event playback audio ke dalam log, termasuk: timestamp, nama jadwal/sumber trigger, file audio yang diputar, audio device target, dan status eksekusi (sukses/gagal beserta pesan error jika ada).
* **FR-5.2 (Prayer Time Sync Logging):** Mencatat hasil sinkronisasi waktu sholat harian: sumber data yang digunakan (API/offline fallback), waktu sholat yang diperoleh, dan error jika terjadi kegagalan fetch.
* **FR-5.3 (Rotating Log File):** Log disimpan ke file di direktori `logs/` dengan mekanisme rotating (maksimal 5 MB per file, 3 file rotasi) untuk mencegah pembengkakan disk.
* **FR-5.4 (Dashboard Health Indicator):** Dashboard menampilkan indikator kesehatan sistem:
  * Status audio device (terdeteksi/tidak terdeteksi).
  * Status konektivitas API waktu sholat (online/offline/fallback aktif).
  * Kapasitas disk tersedia untuk media audio.

### FR-6: System Integration & Desktop Features

* **FR-6.1 (Auto-Start on Windows Boot):** Aplikasi mendaftarkan dirinya ke Windows Startup (via Registry `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`) agar otomatis berjalan saat PC dinyalakan/restart. Fitur ini dapat diaktifkan/nonaktifkan melalui dashboard.
* **FR-6.2 (System Tray Icon):** Saat berjalan, aplikasi menampilkan ikon di Windows System Tray (notification area) dengan menu konteks:
  * **Open Dashboard** — Membuka dashboard di browser default.
  * **Status** — Menampilkan tooltip status (Aktif/Siaga, jadwal berikutnya).
  * **Exit** — Menutup aplikasi.
* **FR-6.3 (Desktop Notification):** Sistem menampilkan notifikasi desktop Windows (toast notification) setiap kali audio diputar, menampilkan nama jadwal dan waktu eksekusi. Berguna sebagai feedback visual jika speaker sedang mati atau volume rendah.

---

## 4. Non-Functional Requirements (NFR)

| Kategori | Spesifikasi |
| --- | --- |
| **Portabilitas & Distribusi** | Terkompilasi sebagai satu file biner tunggal (`.exe`) tanpa ketergantungan pada runtime Python, Node.js, atau instalasi compiler di PC target. File database SQLite dan direktori media berada di luar binary sebagai file pendamping. |
| **Konsumsi Sumber Daya** | Penggunaan memori idle di bawah 75 MB RAM; beban CPU di bawah 2% saat siaga. |
| **Ketahanan & Pemulihan** | Mampu melakukan *auto-recovery* saat crash; data konfigurasi menggunakan SQLite WAL mode untuk menjamin integritas data bahkan saat mati listrik mendadak. |
| **Storage & Data Persistency** | Menggunakan penyimpanan hybrid: `config.json` untuk system settings statis, **SQLite** (`data.db`) untuk data jadwal, konfigurasi sholat, holidays, dan activity log. Tanpa service database eksternal. |
| **UX Otomasi** | Saat file `.exe` dijalankan, sistem otomatis membuka antarmuka pada default web browser sistem operasi dalam waktu kurang dari 2 detik. |
| **Timezone** | Sistem menggunakan konfigurasi timezone eksplisit (default: `Asia/Jakarta`). Jam server ditampilkan secara prominent di dashboard agar admin dapat memverifikasi keakuratan waktu. |

---

## 5. System Architecture & Data Flow

```
┌────────────────────────────────────────────────────────┐
│                   BROWSER (LOCAL / LAN)                │
└───────────────────────────┬────────────────────────────┘
                            │ HTTP (REST API / HTML)
                            ▼
┌────────────────────────────────────────────────────────┐
│            PORTABLE DESKTOP RUNTIME (.EXE)             │
│                                                        │
│  ┌───────────────────────┐  ┌───────────────────────┐  │
│  │   Web Server / API    │  │  Storage Manager      │  │
│  │       (Flask)         │  │  (config.json + SQLite)│  │
│  └───────────┬───────────┘  └───────────▲───────────┘  │
│              │                          │ Reads/Writes │
│              ▼                          │              │
│  ┌──────────────────────────────────────┴───────────┐  │
│  │                SCHEDULER ENGINE                  │  │
│  │           (APScheduler Background Thread)        │  │
│  └───────────┬──────────────────────────────────────┘  │
│              │                                         │
│              ├───────────────────────┐                 │
│              ▼                       ▼                 │
│  ┌───────────────────────┐  ┌───────────────────────┐  │
│  │  Prayer Time Resolver │  │  Audio Engine Player  │  │
│  │ (API + adhanpy fallbk)│  │    (sounddevice +     │  │
│  │                       │  │     soundfile)        │  │
│  └───────────────────────┘  └───────────┬───────────┘  │
│                                         │              │
│  ┌───────────────────────┐              │              │
│  │   System Tray Icon    │              │              │
│  │     (pystray)         │              │              │
│  └───────────────────────┘              │              │
│                                         │              │
│  ┌───────────────────────┐              │              │
│  │  Desktop Notification │              │              │
│  │   (win10toast /       │              │              │
│  │    plyer)             │              │              │
│  └───────────────────────┘              │              │
│                                         │              │
│  ┌───────────────────────┐              │              │
│  │  Activity Logger      │              │              │
│  │ (logging + SQLite +   │              │              │
│  │  RotatingFileHandler) │              │              │
│  └───────────────────────┘              │              │
└─────────────────────────────────────────┼──────────────┘
                                          │ Audio Output
                                          ▼
                             [ Line Out / 3.5mm Jack ]
                                          │
                                          ▼
                               [ PA Central Amplifier ]

```

### 5.1 File & Directory Structure

```
📁 announcer/
├── announcer.exe              ← Binary utama (PyInstaller --onefile)
├── config.json                ← System settings (port, volume, timezone, chime)
├── data.db                    ← SQLite: jadwal, prayer config, holidays, activity log
├── 📁 media/                  ← File audio (.mp3, .wav)
│   ├── chime.mp3
│   ├── adzan_dzuhur.mp3
│   ├── adzan_ashar.mp3
│   ├── adzan_maghrib.mp3
│   ├── pre_adzan.mp3
│   ├── masuk_kantor.mp3
│   └── istirahat.mp3
└── 📁 logs/                   ← Log file rotating
    └── announcer.log
```

---

## 6. Data Schema Specification

### 6.1 System Settings (`config.json`)

Konfigurasi statis yang jarang berubah, disimpan sebagai JSON di root direktori aplikasi:

```json
{
  "system_settings": {
    "port": 5000,
    "timezone": "Asia/Jakarta",
    "volume": 0.85,
    "prepend_chime": true,
    "chime_file": "chime.mp3",
    "audio_device": null,
    "auto_open_browser": true,
    "auto_start_on_boot": true
  }
}
```

| Field | Tipe | Deskripsi |
| --- | --- | --- |
| `port` | integer | Port HTTP untuk dashboard web |
| `timezone` | string | Timezone IANA (default: `Asia/Jakarta`) |
| `volume` | float | Volume global default (0.0 – 1.0) |
| `prepend_chime` | boolean | Bunyikan chime sebelum setiap audio |
| `chime_file` | string | Nama file chime di direktori `media/` |
| `audio_device` | string \| null | Nama audio output device (`null` = default OS) |
| `auto_open_browser` | boolean | Buka browser otomatis saat startup |
| `auto_start_on_boot` | boolean | Daftarkan ke Windows Startup |

### 6.2 Dynamic Data (`data.db` — SQLite)

Data yang sering berubah dan membutuhkan integritas transaksional, disimpan dalam SQLite dengan WAL mode:

#### Tabel: `static_schedules`

| Kolom | Tipe | Deskripsi |
| --- | --- | --- |
| `id` | TEXT PRIMARY KEY | ID unik jadwal (e.g., `sch-01`) |
| `name` | TEXT NOT NULL | Nama pengumuman |
| `time` | TEXT NOT NULL | Jam eksekusi format `HH:mm` |
| `days` | TEXT NOT NULL | JSON array hari aktif (e.g., `["monday","friday"]`) |
| `audio_file` | TEXT NOT NULL | Nama file audio di `media/` |
| `volume` | REAL | Volume override (null = pakai global) |
| `skip_holidays` | INTEGER DEFAULT 1 | 1 = lewati hari libur, 0 = tetap aktif |
| `enabled` | INTEGER DEFAULT 1 | 1 = aktif, 0 = nonaktif |
| `created_at` | TEXT | ISO 8601 timestamp |
| `updated_at` | TEXT | ISO 8601 timestamp |

#### Tabel: `prayer_configuration`

| Kolom | Tipe | Deskripsi |
| --- | --- | --- |
| `key` | TEXT PRIMARY KEY | Nama konfigurasi |
| `value` | TEXT NOT NULL | Nilai (JSON-encoded untuk tipe kompleks) |

Isi default key-value:

| Key | Value (default) |
| --- | --- |
| `enabled` | `true` |
| `city_id` | `"1301"` |
| `city_name` | `"Jakarta"` |
| `latitude` | `-6.2088` |
| `longitude` | `106.8456` |
| `ihtiyat_minutes` | `2` |
| `pre_alert_enabled` | `true` |
| `pre_alert_offset_minutes` | `10` |
| `pre_alert_audio_file` | `"pre_adzan.mp3"` |
| `dzuhur_enabled` | `true` |
| `dzuhur_audio_file` | `"adzan_dzuhur.mp3"` |
| `dzuhur_volume` | `null` |
| `ashar_enabled` | `true` |
| `ashar_audio_file` | `"adzan_ashar.mp3"` |
| `ashar_volume` | `null` |
| `maghrib_enabled` | `true` |
| `maghrib_audio_file` | `"adzan_maghrib.mp3"` |
| `maghrib_volume` | `null` |
| `skip_days` | `["saturday","sunday"]` |

#### Tabel: `holidays`

| Kolom | Tipe | Deskripsi |
| --- | --- | --- |
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | ID otomatis |
| `date` | TEXT NOT NULL UNIQUE | Tanggal libur format `YYYY-MM-DD` |
| `name` | TEXT | Nama hari libur (opsional) |

#### Tabel: `activity_log`

| Kolom | Tipe | Deskripsi |
| --- | --- | --- |
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | ID otomatis |
| `timestamp` | TEXT NOT NULL | ISO 8601 timestamp |
| `event_type` | TEXT NOT NULL | Tipe event: `playback`, `prayer_sync`, `device_check`, `error`, `system` |
| `source` | TEXT | Sumber trigger (nama jadwal, `prayer_dzuhur`, `manual`, `emergency`, dll.) |
| `audio_file` | TEXT | File audio yang diputar (jika relevan) |
| `audio_device` | TEXT | Audio device yang digunakan |
| `status` | TEXT NOT NULL | `success`, `failed`, `skipped` |
| `message` | TEXT | Pesan detail / error message |

---

## 7. Tech Stack Recommendation for Implementation

* **Language Runtime:** Python 3.11+ (stabil untuk bundling Windows).
* **Web Framework:** Flask (minimal footprint, single-file server, mature PyInstaller support).
* **Background Scheduler:** `APScheduler` (`BackgroundScheduler` dengan persistence trigger).
* **Audio Playback:** `sounddevice` + `soundfile` (mendukung audio device enumeration dan pemilihan device spesifik).
* **Prayer Calculation:** `requests` (API MyQuran) + `adhanpy` (kalkulasi offline berbasis astronomi).
* **Frontend UI:** Single-file HTML5 + Tailwind CSS (via CDN/inline) + Vanilla JS / Alpine.js (tanpa Node.js build step).
* **Database:** `sqlite3` (built-in Python stdlib, WAL mode untuk crash-safety).
* **System Tray:** `pystray` (cross-platform system tray icon dengan menu konteks).
* **Desktop Notification:** `plyer` atau `win10toast` (toast notification Windows 10/11).
* **Logging:** `logging` module (built-in Python stdlib) dengan `RotatingFileHandler`.
* **Packager / Compiler:** `PyInstaller` dengan flag `--onefile --noconsole`.

---

## 8. Development Roadmap (Milestones)

### Phase 1: Core Audio & Configuration

* Implementasi modul storage hybrid: pembaca/penyimpan `config.json` + inisialisasi SQLite `data.db` dengan WAL mode, path abstraction (`sys._MEIPASS` / runtime executable directory).
* Implementasi modul audio playback menggunakan `sounddevice` + `soundfile` (pemutaran MP3/WAV, chaining chime, kontrol volume, device enumeration & selection).
* Implementasi Activity Logger (`RotatingFileHandler` + SQLite `activity_log`).

### Phase 2: Dynamic Prayer Time Resolver

* Implementasi integrasi API jadwal sholat dengan mekanisme timeout.
* Pembuatan algoritma fallback lokal menggunakan koordinat GPS via `adhanpy`.
* Implementasi job midnight-sync (`00:01`) untuk meregister alarm sholat harian (Dzuhur, Ashar, Maghrib).
* Logging hasil sinkronisasi (sumber data, waktu sholat, status).

### Phase 3: Web Dashboard & CRUD

* Pembuatan endpoint REST untuk CRUD jadwal statis, konfigurasi sholat, dan upload file audio.
* Pembuatan antarmuka web responsif:
  * Tabel jadwal dengan toggle aktif/nonaktif.
  * Form konfigurasi sholat (3 waktu: Dzuhur, Ashar, Maghrib).
  * Audio device selector (dropdown dari detected devices).
  * Audio file manager dengan preview playback.
  * Tombol emergency broadcast.
  * Activity log viewer (50 entri terakhir).
  * Health indicator (audio device, API status, disk space).
  * Backup & restore configuration (export/import JSON).
* Integrasi timezone display di dashboard.
* Integrasi auto-open browser saat startup aplikasi.

### Phase 4: Desktop Integration & Bundling

* Implementasi System Tray icon dengan menu konteks (`pystray`).
* Implementasi Desktop Notification saat playback (`plyer` / `win10toast`).
* Implementasi Auto-Start on Boot (Windows Registry `HKCU\...\Run`).
* Konfigurasi `.spec` PyInstaller untuk memaketkan static assets (HTML/CSS) dan menyisakan direktori media/config/database di luar binary.
* Pengujian cold-boot di mesin Windows tanpa Python:
  * Verifikasi autorun via Windows Startup.
  * Uji putusnya koneksi internet untuk memvalidasi fallback offline sholat.
  * Uji benturan jadwal audio.
  * Uji audio device disconnect/reconnect.
  * Uji mati listrik mendadak — verifikasi integritas SQLite.