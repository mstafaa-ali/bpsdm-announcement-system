package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata"

	"announcer/internal/audio"
	"announcer/internal/autostart"
	"announcer/internal/config"
	"announcer/internal/database"
	"announcer/internal/paths"
	"announcer/internal/scheduler"
	"announcer/internal/server"
	"announcer/internal/tray"
)

//go:embed templates/*
var embeddedTemplates embed.FS

func findAvailablePort(startPort int) int {
	for port := startPort; port < startPort+50; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err == nil {
			_ = ln.Close()
			return port
		}
	}
	return startPort
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FATAL CRASH] %v\n", r)
			crashLog := filepath.Join(paths.LogsDir, "crash.log")
			_ = os.WriteFile(crashLog, []byte(fmt.Sprintf("FATAL CRASH: %v\n", r)), 0644)
		}
	}()

	fmt.Println("================================================================")
	fmt.Println("  Smart Office Announcement & Prayer Alert System (Go Engine)")
	fmt.Println("  Version 2.0.3 • High Performance & Ultra-Lightweight")
	fmt.Println("================================================================")

	// 1. Initialize persistent runtime storage and database
	if err := paths.InitDirectories(); err != nil {
		log.Fatalf("[Main] Gagal menginisialisasi folder aplikasi: %v", err)
	}

	// Persist logs to disk (crucial for headless Windows GUI apps)
	logFilePath := filepath.Join(paths.LogsDir, "announcer.log")
	if f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, f))
	}

	if err := database.InitDB(); err != nil {
		log.Fatalf("[Main] Gagal menginisialisasi database: %v", err)
	}

	// 2. Load system configurations
	settings := config.GetSettings()
	port := findAvailablePort(settings.Port)
	if port != settings.Port {
		log.Printf("[Main] Port %d sedang digunakan. Beralih ke port %d.\n", settings.Port, port)
	}

	// 3. Configure Windows autostart if requested
	if settings.AutoStartOnBoot {
		autostart.SetAutoStart(true)
	}

	// 4. Start background scheduler (goroutine)
	scheduler.GlobalScheduler.Start()

	// 5. Start web server in background goroutine
	srv := server.NewServer(embeddedTemplates)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: srv.Handler(),
	}

	// Graceful shutdown channel (triggered by tray Quit or OS signal)
	shutdownChan := make(chan struct{})

	go func() {
		database.LogActivity("system", "main", "", "", "success", fmt.Sprintf("Application started on port %d (Go Engine v2.0.3)", port))
		log.Printf("[Main] Dashboard aktif di http://127.0.0.1:%d\n", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Main] Web server error: %v", err)
		}
	}()

	// 6. Auto-open application window after a brief startup delay
	if settings.AutoOpenBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			url := fmt.Sprintf("http://127.0.0.1:%d", port)
			log.Printf("[Main] Membuka jendela aplikasi di %s...\n", url)
			tray.OpenBrowser(url)
		}()
	}

	// 7. OS signal handler in goroutine — handles Ctrl+C / SIGTERM
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("[Main] Menerima sinyal shutdown OS...")
		close(shutdownChan)
	}()

	// 8. Shutdown goroutine — waits for either OS signal or tray quit
	go func() {
		<-shutdownChan
		log.Println("[Main] Memulai graceful shutdown...")
		scheduler.GlobalScheduler.Stop()
		audio.GlobalAudioEngine.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
		os.Exit(0)
	}()

	// 8. Start system tray — this BLOCKS the main thread (required by Win32/Cocoa).
	// On macOS (dev), StartTray is a no-op so main() will block below.
	tray.StartTray(port, func() {
		close(shutdownChan)
	})

	// macOS / Linux fallback: keep the process alive via signal wait
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	scheduler.GlobalScheduler.Stop()
	audio.GlobalAudioEngine.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}
