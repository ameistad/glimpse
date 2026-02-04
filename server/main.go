package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := NewDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	scanner := NewScanner(cfg, db)

	// Start initial scan
	go func() {
		log.Println("Starting initial scan...")
		if err := scanner.Scan(); err != nil {
			log.Printf("Scan error: %v", err)
		}
		log.Println("Initial scan complete")
	}()

	// Start periodic scanner
	go func() {
		ticker := time.NewTicker(cfg.ScanInterval)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Starting periodic scan...")
			if err := scanner.Scan(); err != nil {
				log.Printf("Scan error: %v", err)
			}
			log.Println("Periodic scan complete")
		}
	}()

	// Setup HTTP server
	handler := NewHandler(cfg, db, scanner)
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/photos", handler.ListPhotos)
	mux.HandleFunc("GET /api/photos/{id}", handler.GetPhoto)
	mux.HandleFunc("GET /api/photos/{id}/thumbnail", handler.GetThumbnail)
	mux.HandleFunc("GET /api/photos/{id}/original", handler.GetOriginal)
	mux.HandleFunc("GET /api/folders", handler.ListFolders)
	mux.HandleFunc("GET /api/stats", handler.GetStats)
	mux.HandleFunc("POST /api/scan", handler.TriggerScan)

	corsHandler := corsMiddleware(apiKeyMiddleware(cfg.APIKey, mux))

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: corsHandler,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on %s", cfg.ListenAddr)
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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func apiKeyMiddleware(apiKey string, next http.Handler) http.Handler {
	expected := []byte(apiKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := []byte(r.Header.Get("X-API-Key"))
		if len(expected) == 0 || subtle.ConstantTimeCompare(provided, expected) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
