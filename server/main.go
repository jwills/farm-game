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

type ChatMessage struct {
	ID         int64     `json:"id"`
	PlayerID   string    `json:"playerId"`
	PlayerName string    `json:"playerName"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Gift       *Gift     `json:"gift,omitempty"`
}

type Gift struct {
	CropType   string   `json:"cropType"`
	SizeIndex  int      `json:"sizeIndex"`
	Mutations  []string `json:"mutations"`
	ToPlayer   string   `json:"toPlayer"`
	FromPlayer string   `json:"fromPlayer"`
	Claimed    bool     `json:"claimed"`
	Declined   bool     `json:"declined"`
	Reclaimed  bool     `json:"reclaimed"`
}

type Neighborhood struct {
	Code         string              `json:"code"`
	OwnerID      string              `json:"ownerId"`
	Farms        map[string]FarmData `json:"farms"`
	Messages     []ChatMessage       `json:"messages"`
	MsgID        int64               `json:"msgId"`
	Weather      string              `json:"weather"`
	WeatherUntil time.Time           `json:"weatherUntil"`
	Created      time.Time           `json:"created"`
}

var (
	neighborhoods = make(map[string]*Neighborhood)
	mu            sync.RWMutex
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// API routes
	http.HandleFunc("/api/neighborhood/create", corsMiddleware(handleCreate))
	http.HandleFunc("/api/neighborhood/join", corsMiddleware(handleJoin))
	http.HandleFunc("/api/neighborhood/sync", corsMiddleware(handleSync))
	http.HandleFunc("/api/neighborhood/farms", corsMiddleware(handleGetFarms))
	http.HandleFunc("/api/neighborhood/leave", corsMiddleware(handleLeave))
	http.HandleFunc("/api/neighborhood/kick", corsMiddleware(handleKick))
	http.HandleFunc("/api/neighborhood/steal", corsMiddleware(handleSteal))
	http.HandleFunc("/api/neighborhood/chat/send", corsMiddleware(handleChatSend))
	http.HandleFunc("/api/neighborhood/chat/messages", corsMiddleware(handleChatMessages))
	http.HandleFunc("/api/neighborhood/gift/send", corsMiddleware(handleGiftSend))
	http.HandleFunc("/api/neighborhood/gift/claim", corsMiddleware(handleGiftClaim))
	http.HandleFunc("/api/neighborhood/gift/decline", corsMiddleware(handleGiftDecline))
	http.HandleFunc("/api/neighborhood/gift/reclaim", corsMiddleware(handleGiftReclaim))
	http.HandleFunc("/api/neighborhood/weather", corsMiddleware(handleWeather))

	// Serve static files from parent directory
	fs := http.FileServer(http.Dir("/home/exedev/farm-game"))
	http.Handle("/", fs)

	log.Println("Farm server starting on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
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
		Code:         code,
		OwnerID:      req.PlayerID,
		Farms:        make(map[string]FarmData),
		Messages:     []ChatMessage{},
		MsgID:        0,
		Weather:      "sunny",
		WeatherUntil: time.Now().Add(time.Duration(30+rand.Intn(60)) * time.Second),
		Created:      time.Now(),
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"code": code, "weather": "sunny"})
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

	mu.Lock()
	hood, exists := neighborhoods[code]
	if !exists {
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	// Check if weather needs to change
	if time.Now().After(hood.WeatherUntil) {
		hood.Weather = pickNewWeather(hood.Weather)
		hood.WeatherUntil = time.Now().Add(time.Duration(30+rand.Intn(60)) * time.Second)
	}
	weather := hood.Weather
	weatherUntil := hood.WeatherUntil

	// Return all farms except the requesting player's
	farms := make(map[string]FarmData)
	var myPlots json.RawMessage
	for id, farm := range hood.Farms {
		if id != playerID {
			farms[id] = farm
		} else {
			myPlots = farm.Plots
		}
	}
	mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"farms":        farms,
		"count":        len(farms),
		"weather":      weather,
		"weatherUntil": weatherUntil.Unix(),
		"myPlots":      myPlots,
		"ownerId":      hood.OwnerID,
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

func handleKick(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerID   string `json:"playerId"`
		TargetID   string `json:"targetId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	hood, exists := neighborhoods[req.Code]
	if !exists {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Neighborhood not found"})
		return
	}

	// Only owner can kick
	if hood.OwnerID != req.PlayerID {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Only the neighborhood owner can kick players"})
		return
	}

	// Can't kick yourself
	if req.TargetID == req.PlayerID {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Cannot kick yourself"})
		return
	}

	// Check if target exists
	if _, exists := hood.Farms[req.TargetID]; !exists {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Player not found"})
		return
	}

	delete(hood.Farms, req.TargetID)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleSteal(w http.ResponseWriter, r *http.Request) {
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

	hood, exists := neighborhoods[req.Code]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	// Find all other players with plants
	var victims []string
	for id := range hood.Farms {
		if id != req.PlayerID {
			victims = append(victims, id)
		}
	}

	if len(victims) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"reason":  "no_neighbors",
		})
		return
	}

	// Pick a random victim
	victimID := victims[rand.Intn(len(victims))]
	victim := hood.Farms[victimID]

	// Parse victim's plots to find one with a plant
	var plots []interface{}
	if err := json.Unmarshal(victim.Plots, &plots); err != nil || len(plots) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"reason":  "no_plants",
		})
		return
	}

	// Find non-null plots
	var plantedIndices []int
	for i, p := range plots {
		if p != nil {
			plantedIndices = append(plantedIndices, i)
		}
	}

	if len(plantedIndices) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"reason":  "no_plants",
		})
		return
	}

	// Pick a random plant to steal
	stealIdx := plantedIndices[rand.Intn(len(plantedIndices))]
	stolenPlant := plots[stealIdx]

	// Remove plant from victim
	plots[stealIdx] = nil
	newPlots, _ := json.Marshal(plots)
	victim.Plots = newPlots
	hood.Farms[victimID] = victim

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"plant":      stolenPlant,
		"victimName": victim.PlayerName,
	})
}

func handleChatSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerID   string `json:"playerId"`
		PlayerName string `json:"playerName"`
		Message    string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate message length
	if len(req.Message) == 0 || len(req.Message) > 200 {
		http.Error(w, "Message must be 1-200 characters", http.StatusBadRequest)
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

	hood.MsgID++
	msg := ChatMessage{
		ID:         hood.MsgID,
		PlayerID:   req.PlayerID,
		PlayerName: req.PlayerName,
		Message:    req.Message,
		Timestamp:  time.Now(),
	}

	hood.Messages = append(hood.Messages, msg)

	// Keep only last 50 messages
	if len(hood.Messages) > 50 {
		hood.Messages = hood.Messages[len(hood.Messages)-50:]
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": msg,
	})
}

func handleChatMessages(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	sinceStr := r.URL.Query().Get("since")

	mu.RLock()
	hood, exists := neighborhoods[code]
	mu.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	var messages []ChatMessage
	mu.RLock()
	if sinceStr != "" {
		var sinceID int64
		fmt.Sscanf(sinceStr, "%d", &sinceID)
		for _, msg := range hood.Messages {
			if msg.ID > sinceID {
				messages = append(messages, msg)
			}
		}
	} else {
		messages = hood.Messages
	}
	mu.RUnlock()

	if messages == nil {
		messages = []ChatMessage{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

// Weather types must match frontend WEATHER object keys
var weatherTypes = []string{"sunny", "rainy", "thunder", "windy", "heatwave", "snow"}

func pickNewWeather(current string) string {
	roll := rand.Float64()

	// 5% chance for supercell
	if roll < 0.05 {
		return "supercell"
	}
	// 2.5% chance for Time Anomaly
	if roll < 0.075 {
		return "timeanomaly"
	}
	// 5% chance for hellscape
	if roll < 0.125 {
		return "hellscape"
	}
	// 5% chance for heaven
	if roll < 0.175 {
		return "heaven"
	}

	// Weighted towards sunny
	weighted := []string{"sunny", "sunny", "sunny"}
	weighted = append(weighted, weatherTypes...)

	// Pick random, avoid same weather twice
	for i := 0; i < 10; i++ {
		newWeather := weighted[rand.Intn(len(weighted))]
		if newWeather != current {
			return newWeather
		}
	}
	return "rainy" // Default to rainy (works day or night)
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")

	mu.Lock()
	hood, exists := neighborhoods[code]
	if !exists {
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	// Check if weather needs to change
	if time.Now().After(hood.WeatherUntil) {
		hood.Weather = pickNewWeather(hood.Weather)
		hood.WeatherUntil = time.Now().Add(time.Duration(30+rand.Intn(60)) * time.Second)
	}

	weather := hood.Weather
	weatherUntil := hood.WeatherUntil
	mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"weather":      weather,
		"weatherUntil": weatherUntil.Unix(),
	})
}

func handleGiftSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string   `json:"code"`
		PlayerID   string   `json:"playerId"`
		PlayerName string   `json:"playerName"`
		ToPlayer   string   `json:"toPlayer"`
		CropType   string   `json:"cropType"`
		SizeIndex  int      `json:"sizeIndex"`
		Mutations  []string `json:"mutations"`
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

	// Create gift message
	hood.MsgID++
	msg := ChatMessage{
		ID:         hood.MsgID,
		PlayerID:   req.PlayerID,
		PlayerName: req.PlayerName,
		Message:    fmt.Sprintf("sent a gift to %s!", req.ToPlayer),
		Timestamp:  time.Now(),
		Gift: &Gift{
			CropType:   req.CropType,
			SizeIndex:  req.SizeIndex,
			Mutations:  req.Mutations,
			ToPlayer:   req.ToPlayer,
			FromPlayer: req.PlayerName,
			Claimed:    false,
		},
	}

	hood.Messages = append(hood.Messages, msg)

	// Keep only last 50 messages
	if len(hood.Messages) > 50 {
		hood.Messages = hood.Messages[len(hood.Messages)-50:]
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": msg,
	})
}

func handleGiftClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerName string `json:"playerName"`
		MessageID  int64  `json:"messageId"`
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

	// Find the message with the gift
	for i := range hood.Messages {
		msg := &hood.Messages[i]
		if msg.ID == req.MessageID && msg.Gift != nil {
			if msg.Gift.Claimed {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Gift already claimed",
				})
				return
			}
			if msg.Gift.ToPlayer != req.PlayerName {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "This gift is not for you",
				})
				return
			}
			msg.Gift.Claimed = true
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"gift":    msg.Gift,
			})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   "Gift not found",
	})
}

func handleGiftDecline(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerName string `json:"playerName"`
		MessageID  int64  `json:"messageId"`
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

	// Find the message with the gift
	for i := range hood.Messages {
		msg := &hood.Messages[i]
		if msg.ID == req.MessageID && msg.Gift != nil {
			if msg.Gift.Claimed || msg.Gift.Declined {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Gift already claimed or declined",
				})
				return
			}
			if msg.Gift.ToPlayer != req.PlayerName {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "This gift is not for you",
				})
				return
			}
			msg.Gift.Declined = true
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
			})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   "Gift not found",
	})
}

func handleGiftReclaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerName string `json:"playerName"`
		MessageID  int64  `json:"messageId"`
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

	// Find the message with the gift
	for i := range hood.Messages {
		msg := &hood.Messages[i]
		if msg.ID == req.MessageID && msg.Gift != nil {
			if msg.Gift.Claimed {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Gift already claimed",
				})
				return
			}
			if msg.Gift.Reclaimed {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Gift already reclaimed",
				})
				return
			}
			// Check if requester is the original sender
			if msg.Gift.FromPlayer != req.PlayerName && msg.PlayerName != req.PlayerName {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "You didn't send this gift",
				})
				return
			}
			msg.Gift.Reclaimed = true
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"gift":    msg.Gift,
			})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   "Gift not found",
	})
}
