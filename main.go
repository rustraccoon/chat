// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/shayyz-code/raccoon-sku/backend/contextKeys"
	"github.com/shayyz-code/raccoon-sku/backend/jwt"
	"github.com/shayyz-code/raccoon-sku/backend/limiter"
	"github.com/shayyz-code/raccoon-sku/backend/llama"
)

// 1. Define an interface for our business logic.
// This allows us to use the real `llama` package in production
// and a fake "mock" version in our tests.
type asker interface {
	Ask(prompt llama.Prompt) (string, error)
}

// 2. Create an application struct to hold dependencies.
type application struct {
	ai asker
}

var rl *limiter.RateLimiter

func initRateLimiter(addr *string) {
	rdb := redis.NewClient(&redis.Options{
		Addr: *addr,
	})
	rl = limiter.NewRateLimiter(rdb, 20, 24 * time.Hour) // 20 requests per day
}

func main() {
	
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Could not load .env file.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 3. In main, we create the app with the REAL implementation.
	app := &application{
		ai: llama.LlamaService{}, // Use a struct that implements the interface
	}

	r := mux.NewRouter()


	redisAddr := os.Getenv("REDIS_ADDR")

	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	initRateLimiter(&redisAddr)

	r.HandleFunc("/create-api-key", app.handleCreateAPIKey).Methods("POST")
	// The handler is now a method on our app struct
	// Protect all /api routes
	r.PathPrefix("/api/").Handler(apiMiddleware(http.HandlerFunc(app.apiRouter)))

	log.Println("Server listening on port", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (app *application) apiRouter(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	switch r.URL.Path {
	case "/api/ask-llama":
		app.handleAskLlama(w, r)
	default:
		http.NotFound(w, r)
	}
}

func apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check JWT from Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" || len(auth) < 8 || auth[:7] != "Bearer " {
			http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		token := auth[7:]

		// 2. Parse and verify JWT
		userID, err := jwt.ParseJWT(token)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// 4. Pass user ID to context if needed
		ctx := context.WithValue(r.Context(), contextKeys.UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}


func (app *application) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Header.Get("X-Admin-Token") != os.Getenv("ADMIN_TOKEN") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := r.URL.Query().Get("user")
	if userID == "" {
		http.Error(w, "Missing user", http.StatusBadRequest)
		return
	}

	token, err := jwt.GenerateJWT(userID)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"user":    userID,
		"api_key": token,
	})
}



// handleAskLlama is now a method on `application`.
func (app *application) handleAskLlama(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers to allow requests from any origin.
	// For production, replace "*" with specific origins (e.g., "http://localhost:5173").
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS requests for CORS.
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Ensure the request method is POST.
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	userVal := r.Context().Value(contextKeys.UserIDKey)
	userID, ok := userVal.(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized: no user ID in context", http.StatusUnauthorized)
		return
	}

	// limiter

	allowed, err := rl.Allow(userID)
	if err != nil {
		http.Error(w, "Rate limit check failed", http.StatusInternalServerError)
		return
	}

	if !allowed {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	var req struct {
		SystemPrompt string `json:"systemPrompt"`
		UserPrompt   string `json:"userPrompt"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// It calls the dependency via the interface.
	service := llama.LlamaService{}
	prompt := llama.Prompt{
		System: req.SystemPrompt,
		User:   req.UserPrompt,
	}

	response, err := service.Ask(prompt) // IMPORTANT: Check the error here!

	fmt.Println("Llama Response:", response)
	if err != nil {
		log.Printf("Error from asker: %v", err)
		http.Error(w, "Failed to get response from AI service", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"reply": response,
	})
}