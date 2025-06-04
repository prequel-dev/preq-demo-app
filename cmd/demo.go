package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

var db *sql.DB

func main() {
	// Simple key=value log format
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	initDB()

	// Register HTTP handlers (badjson route removed, new /migrate route added)
	http.Handle("/", loggingMiddleware(http.HandlerFunc(rootHandler)))
	http.Handle("/panic", loggingMiddleware(http.HandlerFunc(panicHandler)))
	http.Handle("/slow", loggingMiddleware(http.HandlerFunc(slowHandler)))
	http.Handle("/migrate", loggingMiddleware(http.HandlerFunc(migrationHandler)))
	http.Handle("/health", http.HandlerFunc(healthHandler))

	addr := ":8080"
	log.Printf("level=info msg=\"starting server\" addr=%s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("level=fatal msg=\"server exited\" err=%v", err)
	}
}

// initDB opens an in‑memory SQLite database used solely to demonstrate migration failures.
func initDB() {
	var err error
	db, err = sql.Open("sqlite", "file:demo.db?cache=shared&mode=memory")
	if err != nil {
		log.Fatalf("level=fatal msg=\"failed to open db\" err=%v", err)
	}
}

// loggingMiddleware logs request/response metadata in a uniform format.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)
		log.Printf("level=info method=%s path=%s status=%d duration=%s", r.Method, r.URL.Path, lrw.statusCode, duration)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// rootHandler returns a basic JSON payload.
func rootHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"message": "demo service"})
}

// panicHandler triggers a panic inside a goroutine. The goroutine recovers so the service stays up.
func panicHandler(w http.ResponseWriter, r *http.Request) {
	go func() {
		panic("intentional panic inside goroutine for demo purposes")
	}()
	respondJSON(w, http.StatusOK, map[string]string{"status": "goroutine panic triggered"})
}

// slowHandler simulates a slow request and logs if the client cancels.
func slowHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	select {
	case <-time.After(6 * time.Second):
		respondJSON(w, http.StatusOK, map[string]string{"status": "slow response"})
	case <-ctx.Done():
		log.Printf("level=error msg=\"context canceled\" path=%s err=%v", r.URL.Path, ctx.Err())
	}
}

// migrationHandler deliberately runs a faulty SQL migration to demonstrate error logging.
func migrationHandler(w http.ResponseWriter, r *http.Request) {
	if err := runFaultyMigration(); err != nil {
		log.Printf("level=error msg=\"migration failed\" err=%v", err)
		http.Error(w, "migration failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "migration succeeded (unexpected)"})
}

func runFaultyMigration() error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	log.Println("level=info msg=\"running migration\"")

	// Intentional error: altering a non‑existent table
	if _, err := tx.Exec("ALTER TABLE imaginary ADD COLUMN foo TEXT"); err != nil {
		return fmt.Errorf("alter table: %w", err)
	}

	return tx.Commit()
}

// healthHandler is a quiet liveness probe.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// respondJSON writes a JSON response and logs encoding failures.
func respondJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("level=error msg=\"failed to encode json\" err=%v payload=%#v", err, payload)
	}
}
