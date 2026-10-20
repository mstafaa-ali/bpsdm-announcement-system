@echo off
title Smart Office Announcer - Go Windows Builder
echo ================================================================
echo   Smart Office Announcement & Prayer Alert System
echo   Build Standalone Windows .EXE with Go (Zero Dependency)
echo ================================================================
echo.

:: 1. Check Go
where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Compiler Go tidak ditemukan di sistem ini.
    echo Silakan install Go dari https://go.dev/dl/ terlebih dahulu.
    pause
    exit /b 1
)

echo [1/3] Membangun binary Windows (SmartOfficeAnnouncer.exe)...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w -H=windowsgui" -o Release_App_Go\SmartOfficeAnnouncer.exe .

if errorlevel 1 (
    echo [ERROR] Kompilasi gagal!
    pause
    exit /b 1
)

echo [2/3] Memeriksa UPX compressor...
where upx >nul 2>&1
if not errorlevel 1 (
    echo   Menjalankan kompresi UPX...
    upx --best --lzma Release_App_Go\SmartOfficeAnnouncer.exe
)

echo [3/3] Menyiapkan paket portable Release_App_Go...
if not exist "Release_App_Go\media" mkdir "Release_App_Go\media"
copy /Y config.json Release_App_Go\ >nul 2>&1
if exist "media" xcopy /E /I /Y "media" "Release_App_Go\media" >nul 2>&1

echo.
echo ================================================================
echo   [SUKSES] Binary berhasil dibuat!
echo   Folder: Release_App_Go\
echo   Executable: Release_App_Go\SmartOfficeAnnouncer.exe
echo ================================================================
echo.
pause
