package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type tokenState struct {
	mu       sync.RWMutex
	yva      string
	userID   string
	exp      time.Time
	lastPage int
}

var state = &tokenState{}

func (s *tokenState) set(yva, userID string, exp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.yva = yva
	s.userID = userID
	s.exp = exp
	s.lastPage = 0
}

func (s *tokenState) get() (yva, userID string, exp time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.yva, s.userID, s.exp
}

func (s *tokenState) getLastPage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPage
}

func (s *tokenState) setLastPage(p int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPage = p
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	rawToken := os.Getenv("YVA_TOKEN")
	if rawToken == "" {
		log.Fatal("YVA_TOKEN environment variable is required.\n" +
			"  1. Go to bible.com and sign in\n" +
			"  2. Open DevTools → Application → Cookies → www.bible.com\n" +
			"  3. Copy the value of the 'yva' cookie\n" +
			"  4. Set YVA_TOKEN=<value> in your .env file")
	}

	if err := applyToken(rawToken); err != nil {
		log.Fatalf("Invalid YVA_TOKEN: %v", err)
	}

	go discoverLastPage()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/random", handleRandom)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/refresh", handleRefresh)

	log.Printf("Listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func discoverLastPage() {
	yva, userID, _ := state.get()
	log.Printf("Discovering total number of highlight pages...")
	last, err := findLastPage(yva, userID)
	if err != nil {
		log.Printf("Could not discover last page: %v", err)
		return
	}
	state.setLastPage(last)
	log.Printf("Found %d pages of highlights (~%d total)", last, last*25)
}

func applyToken(raw string) error {
	userID, exp, err := parseToken(raw)
	if err != nil {
		return err
	}
	state.set(raw, userID, exp)
	if tokenIsExpired(exp) {
		log.Printf("WARNING: token expired at %s — go to /refresh to update it", exp.Format("2006-01-02 15:04"))
	} else {
		log.Printf("Ready — user %s, token valid until %s", userID, exp.Format("2006-01-02 15:04"))
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	_, _, exp := state.get()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"expired":    tokenIsExpired(exp),
		"expires_at": exp.Format(time.RFC3339),
		"expires_in": fmt.Sprintf("%.0f hours", time.Until(exp).Hours()),
	})
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		newToken := r.FormValue("token")
		if newToken == "" {
			http.Error(w, "token is required", http.StatusBadRequest)
			return
		}
		if err := applyToken(newToken); err != nil {
			http.Error(w, "Invalid token: "+err.Error(), http.StatusBadRequest)
			return
		}
		go discoverLastPage()
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, "static/refresh.html")
}

func handleRandom(w http.ResponseWriter, r *http.Request) {
	yva, userID, exp := state.get()
	if tokenIsExpired(exp) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error":       "token expired",
			"refresh_url": "/refresh",
		})
		return
	}

	highlight, err := enrichHighlight(yva, userID, state.getLastPage())
	if err != nil {
		log.Printf("error fetching highlight: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, highlight)
}
