@echo off
title Smart Office Announcer Runner
echo ================================================================
echo   Smart Office Announcement & Prayer Alert System (Go Engine)
echo ================================================================
echo.

if exist "Release_App_Go\SmartOfficeAnnouncer.exe" (
    echo Menjalankan Release_App_Go\SmartOfficeAnnouncer.exe...
    start "" "Release_App_Go\SmartOfficeAnnouncer.exe"
    exit /b 0
)

if exist "SmartOfficeAnnouncer.exe" (
    echo Menjalankan SmartOfficeAnnouncer.exe...
    start "" "SmartOfficeAnnouncer.exe"
    exit /b 0
)

where go >nul 2>&1
if not errorlevel 1 (
    echo Menjalankan mode development via 'go run main.go'...
    go run main.go
    exit /b 0
)

echo [ERROR] File executable SmartOfficeAnnouncer.exe tidak ditemukan.
echo Silakan jalankan build_go_windows.bat terlebih dahulu untuk membangun binary.
echo.
pause
