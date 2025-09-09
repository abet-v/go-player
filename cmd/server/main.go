package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"GoPlayer/internal/server"
)

func main() {
	mux := http.NewServeMux()

	rm := server.NewRoomManager()

	// Static files
	webDir := http.Dir(filepath.Clean("web"))
	fileServer := http.FileServer(webDir)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Create a new room
	mux.HandleFunc("/api/new", func(w http.ResponseWriter, r *http.Request) {
		id := randID()
		rm.CreateRoom(id)
		http.Redirect(w, r, "/room/"+id, http.StatusFound)
	})

	// WebSocket endpoint: /ws/{roomID}
	mux.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) {
		// Expect path /ws/{id}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			http.Error(w, "missing room id", http.StatusBadRequest)
			return
		}
		roomID := parts[0]
		rm.ServeWS(roomID, w, r)
	})

	// Room page: serve index.html; frontend will read room id from URL
	mux.HandleFunc("/room/", func(w http.ResponseWriter, r *http.Request) {
		// always serve index.html; SPA-like
		http.ServeFile(w, r, filepath.Join("web", "index.html"))
	})

	// Root -> new room landing page with a button to create
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// static asset under web/
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join("web", "landing.html"))
	})

	// Bind to the configured port (Fly sets PORT env)
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Go/Baduk server running at %s\n", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start)
		log.Printf("%s %s %s %v", r.RemoteAddr, r.Method, r.URL.Path, dur)
	})
}

func randID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
