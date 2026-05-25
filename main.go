package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	devMode := flag.Bool("dev", os.Getenv("GLIMPSE_DEV") == "1", "Enable development-only hot reload helpers")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg.Development = *devMode

	db, err := NewDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	scanner := NewScanner(cfg, db)

	go func() {
		log.Println("Starting initial scan...")
		if err := scanner.Scan(); err != nil && !errors.Is(err, ErrScanAlreadyRunning) {
			log.Printf("Scan error: %v", err)
		}
		log.Println("Initial scan complete")
	}()

	go func() {
		ticker := time.NewTicker(cfg.ScanInterval)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Starting periodic scan...")
			if err := scanner.Scan(); err != nil && !errors.Is(err, ErrScanAlreadyRunning) {
				log.Printf("Scan error: %v", err)
			}
			log.Println("Periodic scan complete")
		}
	}()

	handler, err := NewHandler(cfg, db, scanner)
	if err != nil {
		log.Fatalf("Failed to initialize web handler: %v", err)
	}

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: handler.Routes(),
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Glimpse starting on %s", listenURL(cfg.ListenAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}

func listenURL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return "http://localhost" + addr
		}
		return "http://" + addr
	}

	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}
