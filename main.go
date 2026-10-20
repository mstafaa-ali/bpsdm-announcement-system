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
	fmt.Println("================================================================")
	fmt.Println("  Smart Office Announcement & Prayer Alert System (Go Engine)")
	fmt.Println("  Version 2.0.0 • High Performance & Ultra-Lightweight")
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

	// 4. Start background scheduler
	scheduler.GlobalScheduler.Start()

	// 5. Start system tray
	shutdownChan := make(chan struct{})
	tray.StartTray(port, func() {
		close(shutdownChan)
	})

	// 6. Fast auto-open application window
	if settings.AutoOpenBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			url := fmt.Sprintf("http://127.0.0.1:%d", port)
			log.Printf("[Main] Membuka jendela aplikasi di %s...\n", url)
			tray.OpenBrowser(url)
		}()
	}

	// 7. Start Web Server
	srv := server.NewServer(embeddedTemplates)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: srv.Handler(),
	}

	database.LogActivity("system", "main", "", "", "success", fmt.Sprintf("Application started on port %d (Go Engine)", port))
	log.Printf("[Main] Dashboard aktif di http://127.0.0.1:%d\n", port)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			log.Println("[Main] Menerima sinyal shutdown...")
		case <-shutdownChan:
			log.Println("[Main] Menutup aplikasi dari menu system tray...")
		}

		scheduler.GlobalScheduler.Stop()
		audio.GlobalAudioEngine.Stop()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
		os.Exit(0)
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[Main] Web server error: %v", err)
	}
}
