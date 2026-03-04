package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const neighborhoodsFile = "neighborhoods.json"

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
	CropType   string   `json:"cropType,omitempty"`
	SizeIndex  int      `json:"sizeIndex,omitempty"`
	Mutations  []string `json:"mutations,omitempty"`
	PetId      string   `json:"petId,omitempty"`
	SeedType   string   `json:"seedType,omitempty"`   // For admin give command
	SeedQty    int      `json:"seedQty,omitempty"`    // For admin give command
	GearId     string   `json:"gearId,omitempty"`     // For admin give command
	Money      int64    `json:"money,omitempty"`      // For admin give command
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
	// Admin player names (case-insensitive check)
	adminNames = map[string]bool{
		"iamweird": true,
	}
)

func isAdmin(playerName string) bool {
	return adminNames[strings.ToLower(playerName)]
}

// loadNeighborhoods loads neighborhood data from the JSON file
func loadNeighborhoods() {
	data, err := os.ReadFile(neighborhoodsFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No neighborhoods file found, starting fresh")
			return
		}
		log.Printf("Error reading neighborhoods file: %v", err)
		return
	}

	var loaded map[string]*Neighborhood
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("Error parsing neighborhoods file: %v", err)
		return
	}

	neighborhoods = loaded
	log.Printf("Loaded %d neighborhoods from file", len(neighborhoods))
}

// saveNeighborhoods saves all neighborhood data to the JSON file
func saveNeighborhoods() {
	data, err := json.MarshalIndent(neighborhoods, "", "  ")
	if err != nil {
		log.Printf("Error marshaling neighborhoods: %v", err)
		return
	}

	if err := os.WriteFile(neighborhoodsFile, data, 0644); err != nil {
		log.Printf("Error saving neighborhoods: %v", err)
		return
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Load existing neighborhoods from file
	loadNeighborhoods()

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
	http.HandleFunc("/api/neighborhood/gift/pet/send", corsMiddleware(handlePetGiftSend))
	http.HandleFunc("/api/neighborhood/gift/pet/claim", corsMiddleware(handlePetGiftClaim))
	http.HandleFunc("/api/neighborhood/weather", corsMiddleware(handleWeather))
	http.HandleFunc("/api/admin/command", corsMiddleware(handleAdminCommand))
	http.HandleFunc("/api/admin/check", corsMiddleware(handleAdminCheck))

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
		WeatherUntil: time.Now().Add(getWeatherDuration("sunny")),
		Created:      time.Now(),
	}

	saveNeighborhoods()
	json.NewEncoder(w).Encode(map[string]interface{}{"code": code, "weather": "sunny", "ownerId": req.PlayerID})
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

	mu.Lock()
	defer mu.Unlock()

	// Auto-create neighborhood 7655 if it doesn't exist (global neighborhood)
	if req.Code == "7655" {
		if _, exists := neighborhoods["7655"]; !exists {
			neighborhoods["7655"] = &Neighborhood{
				Code:         "7655",
				OwnerID:      "player_y8nykf1y1", // iamweird
				Farms:        make(map[string]FarmData),
				Messages:     []ChatMessage{},
				MsgID:        0,
				Weather:      "sunny",
				WeatherUntil: time.Now().Add(getWeatherDuration("sunny")),
				Created:      time.Now(),
			}
		}
	}

	hood, exists := neighborhoods[req.Code]

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Neighborhood not found"})
		return
	}

	saveNeighborhoods()
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

	saveNeighborhoods()
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
	weatherChanged := false
	if time.Now().After(hood.WeatherUntil) {
		hood.Weather = pickNewWeather(hood.Weather)
		hood.WeatherUntil = time.Now().Add(getWeatherDuration(hood.Weather))
		weatherChanged = true
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
	if weatherChanged {
		saveNeighborhoods()
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
		saveNeighborhoods()
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
	saveNeighborhoods()
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

	saveNeighborhoods()
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

	saveNeighborhoods()
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

	// 7.5% chance for supercell
	if roll < 0.075 {
		return "supercell"
	}
	// 2.5% chance for Time Anomaly
	if roll < 0.10 {
		return "timeanomaly"
	}
	// 5% chance for hellscape
	if roll < 0.15 {
		return "hellscape"
	}
	// 5% chance for heaven
	if roll < 0.20 {
		return "heaven"
	}
	// 20% chance for disco
	if roll < 0.40 {
		return "disco"
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

// getWeatherDuration returns the duration for a given weather type
func getWeatherDuration(weather string) time.Duration {
	switch weather {
	case "supercell", "timeanomaly":
		// 60-120 seconds for special weather
		return time.Duration(60+rand.Intn(60)) * time.Second
	case "disco":
		// 90-180 seconds for disco (1.5-3 minutes)
		return time.Duration(90+rand.Intn(90)) * time.Second
	default:
		// 10-30 seconds for normal weather
		return time.Duration(10+rand.Intn(20)) * time.Second
	}
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
	weatherChanged := false
	if time.Now().After(hood.WeatherUntil) {
		hood.Weather = pickNewWeather(hood.Weather)
		hood.WeatherUntil = time.Now().Add(getWeatherDuration(hood.Weather))
		weatherChanged = true
	}

	weather := hood.Weather
	weatherUntil := hood.WeatherUntil
	if weatherChanged {
		saveNeighborhoods()
	}
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

	saveNeighborhoods()
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
			saveNeighborhoods()
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
			saveNeighborhoods()
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
			saveNeighborhoods()
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


func handlePetGiftSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerID   string `json:"playerId"`
		PlayerName string `json:"playerName"`
		ToPlayer   string `json:"toPlayer"`
		PetId      string `json:"petId"`
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

	// Create pet gift message
	hood.MsgID++
	msg := ChatMessage{
		ID:         hood.MsgID,
		PlayerID:   req.PlayerID,
		PlayerName: req.PlayerName,
		Message:    fmt.Sprintf("sent a pet to %s!", req.ToPlayer),
		Timestamp:  time.Now(),
		Gift: &Gift{
			PetId:      req.PetId,
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

	saveNeighborhoods()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": msg,
	})
}

func handlePetGiftClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerName string `json:"playerName"`
		MessageID  int64  `json:"messageId"`
		PetCount   int    `json:"petCount"` // How many of this pet the claimer already has
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

	// Find the message with the pet gift
	for i := range hood.Messages {
		msg := &hood.Messages[i]
		if msg.ID == req.MessageID && msg.Gift != nil && msg.Gift.PetId != "" {
			if msg.Gift.Claimed {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Pet already claimed",
				})
				return
			}
			if msg.Gift.Declined {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Pet was declined",
				})
				return
			}
			if msg.Gift.Reclaimed {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Pet was reclaimed",
				})
				return
			}
			// Check if this gift is for the requester
			if msg.Gift.ToPlayer != req.PlayerName {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "This pet is not for you",
				})
				return
			}
			// Check if recipient already has 3 of this pet
			if req.PetCount >= 3 {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "You already have 3 of this pet!",
				})
				return
			}
			msg.Gift.Claimed = true
			saveNeighborhoods()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"gift":    msg.Gift,
			})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   "Pet gift not found",
	})
}

// handleAdminCheck checks if a player is an admin
func handleAdminCheck(w http.ResponseWriter, r *http.Request) {
	playerName := r.URL.Query().Get("playerName")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isAdmin": isAdmin(playerName),
	})
}

// handleAdminCommand handles admin commands
func handleAdminCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code       string `json:"code"`
		PlayerName string `json:"playerName"`
		Command    string `json:"command"`
		Args       string `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Check if player is admin
	if !isAdmin(req.PlayerName) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Not authorized",
		})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	hood, exists := neighborhoods[req.Code]
	if !exists {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Neighborhood not found",
		})
		return
	}

	switch req.Command {
	case "setweather":
		// Valid weather types
		validWeathers := map[string]bool{
			"sunny": true, "cloudy": true, "rainy": true, "windy": true,
			"foggy": true, "thunder": true, "snow": true, "heatwave": true,
			"bloodmoon": true, "hellscape": true, "heaven": true,
			"supercell": true, "timeanomaly": true, "disco": true,
			"partlycloudy": true, "drizzle": true,
		}
		weather := strings.ToLower(strings.TrimSpace(req.Args))
		if !validWeathers[weather] {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid weather type: " + req.Args,
			})
			return
		}
		hood.Weather = weather
		hood.WeatherUntil = time.Now().Add(getWeatherDuration(weather))
		saveNeighborhoods()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Weather set to " + weather,
		})

	case "extendweather":
		// Extend current weather by 5 minutes
		hood.WeatherUntil = hood.WeatherUntil.Add(5 * time.Minute)
		saveNeighborhoods()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Weather extended by 5 minutes",
		})

	case "broadcast":
		// Send a system message to chat
		if req.Args == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Message required",
			})
			return
		}
		hood.MsgID++
		msg := ChatMessage{
			ID:         hood.MsgID,
			PlayerID:   "SYSTEM",
			PlayerName: "📢 ADMIN",
			Message:    req.Args,
			Timestamp:  time.Now(),
		}
		hood.Messages = append(hood.Messages, msg)
		if len(hood.Messages) > 50 {
			hood.Messages = hood.Messages[len(hood.Messages)-50:]
		}
		saveNeighborhoods()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Broadcast sent",
		})

	case "kick":
		// Kick a player by name
		targetName := strings.TrimSpace(req.Args)
		if targetName == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Player name required",
			})
			return
		}
		for pid, farm := range hood.Farms {
			if strings.EqualFold(farm.PlayerName, targetName) {
				delete(hood.Farms, pid)
				saveNeighborhoods()
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": "Kicked " + targetName,
				})
				return
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Player not found: " + targetName,
		})

	case "listplayers":
		var players []string
		for _, farm := range hood.Farms {
			players = append(players, farm.PlayerName)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"players": players,
		})

	case "give":
		// /give <player> <type> <item> [quantity]
		// Types: seed, pet, gear, money
		// Examples:
		//   /give joe seed wheat 10
		//   /give joe pet dragon
		//   /give joe gear sprinkler_prismatic
		//   /give joe money 1000000
		parts := strings.Fields(req.Args)
		if len(parts) < 3 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Usage: /give <player> <type> <item> [quantity]",
				"examples": []string{
					"/give joe seed wheat 10",
					"/give joe pet dragon",
					"/give joe gear sprinkler_prismatic",
					"/give joe money 1000000",
				},
			})
			return
		}

		targetPlayer := parts[0]
		giveType := strings.ToLower(parts[1])
		itemId := strings.ToLower(parts[2])

		// Find if target player exists in neighborhood
		found := false
		for _, farm := range hood.Farms {
			if strings.EqualFold(farm.PlayerName, targetPlayer) {
				targetPlayer = farm.PlayerName // Use exact name
				found = true
				break
			}
		}
		if !found {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Player not found in neighborhood: " + targetPlayer,
			})
			return
		}

		var gift *Gift
		var giftDesc string

		switch giveType {
		case "seed", "seeds":
			qty := 1
			if len(parts) >= 4 {
				if q, err := strconv.Atoi(parts[3]); err == nil && q > 0 {
					qty = q
				}
			}
			gift = &Gift{
				SeedType:   itemId,
				SeedQty:    qty,
				ToPlayer:   targetPlayer,
				FromPlayer: "🛡️ Admin",
			}
			giftDesc = fmt.Sprintf("%d %s seeds", qty, itemId)

		case "pet", "pets":
			gift = &Gift{
				PetId:      itemId,
				ToPlayer:   targetPlayer,
				FromPlayer: "🛡️ Admin",
			}
			giftDesc = itemId + " pet"

		case "gear":
			gift = &Gift{
				GearId:     itemId,
				ToPlayer:   targetPlayer,
				FromPlayer: "🛡️ Admin",
			}
			giftDesc = itemId + " gear"

		case "money", "cash", "coins":
			amount, err := strconv.ParseInt(itemId, 10, 64)
			if err != nil || amount <= 0 {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Invalid money amount",
				})
				return
			}
			gift = &Gift{
				Money:      amount,
				ToPlayer:   targetPlayer,
				FromPlayer: "🛡️ Admin",
			}
			giftDesc = fmt.Sprintf("$%d", amount)

		default:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Unknown gift type: " + giveType,
				"types":   []string{"seed", "pet", "gear", "money"},
			})
			return
		}

		// Create gift message in chat
		hood.MsgID++
		msg := ChatMessage{
			ID:         hood.MsgID,
			PlayerID:   "ADMIN",
			PlayerName: "🛡️ Admin",
			Message:    fmt.Sprintf("sent %s to %s!", giftDesc, targetPlayer),
			Timestamp:  time.Now(),
			Gift:       gift,
		}
		hood.Messages = append(hood.Messages, msg)
		if len(hood.Messages) > 50 {
			hood.Messages = hood.Messages[len(hood.Messages)-50:]
		}

		saveNeighborhoods()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Sent %s to %s", giftDesc, targetPlayer),
		})

	default:
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unknown command: " + req.Command,
			"commands": []string{
				"/setweather <type> - Set weather (sunny, disco, supercell, etc)",
				"/extendweather - Extend current weather by 5 min",
				"/broadcast <msg> - Send system message",
				"/kick <player> - Kick a player",
				"/listplayers - List all players",
				"/give <player> <type> <item> [qty] - Give items (seed/pet/gear/money)",
			},
		})
	}
}
