@echo off
title Smart Office Announcer - Windows Setup & Builder
echo ================================================================
echo   Smart Office Announcement & Prayer Alert System
echo   Setup & Build Standalone .EXE for Windows
echo ================================================================
echo.

:: 1. Check Python
python --version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Python tidak terdeteksi di sistem ini.
    echo Silakan install Python 3.11 atau 3.12 dari python.org terlebih dahulu.
    echo Pastikan centang "Add Python to PATH" saat instalasi.
    pause
    exit /b 1
)

:: 2. Create Virtual Environment if not exists
if not exist "venv" (
    echo [1/3] Membuat Virtual Environment Python (venv)...
    python -m venv venv
) else (
    echo [1/3] Virtual Environment (venv) sudah ada.
)

:: 3. Install Dependencies
echo [2/3] Menginstall dependensi requirements.txt...
call venv\Scripts\activate.bat
python -m pip install --upgrade pip
pip install -r requirements.txt

:: 4. Build Standalone EXE using PyInstaller
echo.
echo [3/3] Membangun file executable tunggal (SmartOfficeAnnouncer.exe)...
pyinstaller build_windows.spec

if exist "dist\SmartOfficeAnnouncer.exe" (
    echo.
    echo ================================================================
    echo   [SUKSES] Binary .EXE berhasil dibuat!
    echo   Lokasi: dist\SmartOfficeAnnouncer.exe
    echo ================================================================
    echo.
    echo Menyiapkan folder siap pakai "Release_App"...
    if not exist "Release_App" mkdir "Release_App"
    copy /Y "dist\SmartOfficeAnnouncer.exe" "Release_App\"
    copy /Y "config.json" "Release_App\"
    if exist "media" xcopy /E /I /Y "media" "Release_App\media"
    echo.
    echo Folder "Release_App" sudah siap dipindahkan ke mana saja (Portable).
) else (
    echo.
    echo [ERROR] Gagal membuat file .exe. Silakan periksa log di atas.
)

echo.
pause
