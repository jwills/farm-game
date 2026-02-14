package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type FarmData struct {
	PlayerName string          `json:"playerName"`
	Plots      json.RawMessage `json:"plots"`
	Money      int             `json:"money"`
	Pets       []string        `json:"pets"`
	LastSync   time.Time       `json:"lastSync"`
}

type Neighborhood struct {
	Code    string              `json:"code"`
	Farms   map[string]FarmData `json:"farms"`
	Created time.Time           `json:"created"`
}

var (
	neighborhoods = make(map[string]*Neighborhood)
	mu            sync.RWMutex
)

func main() {
	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/api/neighborhood/create", corsMiddleware(handleCreate))
	http.HandleFunc("/api/neighborhood/join", corsMiddleware(handleJoin))
	http.HandleFunc("/api/neighborhood/sync", corsMiddleware(handleSync))
	http.HandleFunc("/api/neighborhood/farms", corsMiddleware(handleGetFarms))
	http.HandleFunc("/api/neighborhood/leave", corsMiddleware(handleLeave))

	log.Println("Neighborhood server starting on :8001")
	log.Fatal(http.ListenAndServe(":8001", nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func generateCode() string {
	return fmt.Sprintf("%04d", rand.Intn(10000))
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PlayerID   string `json:"playerId"`
		PlayerName string `json:"playerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Generate unique code
	var code string
	for {
		code = generateCode()
		if _, exists := neighborhoods[code]; !exists {
			break
		}
	}

	neighborhoods[code] = &Neighborhood{
		Code:    code,
		Farms:   make(map[string]FarmData),
		Created: time.Now(),
	}

	json.NewEncoder(w).Encode(map[string]string{"code": code})
}

func handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerID   string `json:"playerId"`
		PlayerName string `json:"playerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.RLock()
	hood, exists := neighborhoods[req.Code]
	mu.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"code":    hood.Code,
		"members": len(hood.Farms),
	})
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string          `json:"code"`
		PlayerID   string          `json:"playerId"`
		PlayerName string          `json:"playerName"`
		Plots      json.RawMessage `json:"plots"`
		Money      int             `json:"money"`
		Pets       []string        `json:"pets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	hood, exists := neighborhoods[req.Code]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	hood.Farms[req.PlayerID] = FarmData{
		PlayerName: req.PlayerName,
		Plots:      req.Plots,
		Money:      req.Money,
		Pets:       req.Pets,
		LastSync:   time.Now(),
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleGetFarms(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	playerID := r.URL.Query().Get("playerId")

	mu.RLock()
	hood, exists := neighborhoods[code]
	mu.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	// Return all farms except the requesting player's
	farms := make(map[string]FarmData)
	mu.RLock()
	for id, farm := range hood.Farms {
		if id != playerID {
			farms[id] = farm
		}
	}
	mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"farms": farms,
		"count": len(farms),
	})
}

func handleLeave(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code     string `json:"code"`
		PlayerID string `json:"playerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if hood, exists := neighborhoods[req.Code]; exists {
		delete(hood.Farms, req.PlayerID)
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
