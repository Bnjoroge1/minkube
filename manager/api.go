package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// bearerToken extracts the content from the header, striping the Bearer prefix
type APIkeysConfig struct {
	APIKeys map[string]APIKeyInfo `json:"api_keys"`
}
type APIKeyInfo struct {
	Permissions []string `json:"permissions"`
	Description string   `json:"description"`
	CurrentRate int      `json:"current_rate"`
	RateLimits  int      `json:"rate_limits"`
	IsActive    bool     `json:"is_active"`
	ExpiresAt   string   `json:"expires_at"`
}

type Api struct {
	Address   string
	Port      int
	Manager   *Manager
	Router    *chi.Mux
	APIConfig APIkeysConfig
}

var endpointPermissions = map[string]string{
	"POST /tasks":     "tasks:write",
	"GET /tasks":      "tasks:read",
	"DELETE /tasks/*": "tasks:write",
}
var endpointRateLimits = map[string]int{
	"POST /tasks":     400,
	"GET /tasks":      5000,
	"DELETE /tasks/*": 400,
}

func (a *Api) keyIsActive(apiKey string) (bool, error) {
	keyInfo, exists := a.APIConfig.APIKeys[apiKey]
	if !exists {
		return false, errors.New("API key does not exist")
	}
	if !keyInfo.IsActive {
		return false, errors.New("API key is inactive")
	}
	return true, nil
}
func (a *Api) bearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("Missing Authorization header")
	}
	pieces := strings.SplitN(authHeader, " ", 2)

	if len(pieces) < 2 || pieces[0] != "Bearer" {
		return "", errors.New("token with incorrect bearer format")
	}

	token := strings.TrimSpace(pieces[1])
	if token == "" {
		return "", errors.New("bearer token is empty")
	}

	return token, nil
}
func (a *Api) ApiKeyHasPermissions(apiKey string, requiredPermission string) (bool, error) {
	keyInfo, exists := a.APIConfig.APIKeys[apiKey]
	if !exists {
		return false, errors.New("API key not found")
	}

	//check if api key has required permissions
	for _, permission := range keyInfo.Permissions {
		if permission == requiredPermission || permission == "admin:*" {
			return true, nil
		}

	}
	return false, errors.New("Insufficient permissions")

}

func (a *Api) getRateLimits(r *http.Request) int {
	method := r.Method
	path := r.URL.Path
	methodPath := method + " " + path
	// Handle parameterized routes like DELETE /tasks/{id}
	if method == "DELETE" && strings.HasPrefix(path, "/tasks/") {
		methodPath = "DELETE /tasks"
	}
	return endpointRateLimits[methodPath]
}
func (a *Api) belowRateLimit(apikey string, requiredRateLimit int) (bool, error) {
	keyInfo, exists := a.APIConfig.APIKeys[apikey]
	if !exists {
		return false, errors.New("API key does not exist")
	}
	//check if apikey's current rate limit is below

	if keyInfo.CurrentRate < requiredRateLimit {
		return true, nil
	}
	return false, errors.New("API key currently is above the allowed rate limit")

}
func (a *Api) getRequiredPermission(r *http.Request) string {
	method := r.Method
	path := r.URL.Path
	methodPath := method + " " + path
	// Handle parameterized routes like DELETE /tasks/{id}
	if method == "DELETE" && strings.HasPrefix(path, "/tasks/") {
		methodPath = "DELETE /tasks"
	}
	return endpointPermissions[methodPath]
}
func (a *Api) ApiRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey, err := a.bearerToken(r)
		if err != nil {
			log.Printf("Cannot extract API key")
			w.WriteHeader(http.StatusUnauthorized)
			e := ErrResponse{
				HTTPStatusCode: http.StatusUnauthorized,
				Message:        "Invalid or missing unauthorized api key",
			}
			json.NewEncoder(w).Encode(e)
			return
		}
		active, err := a.keyIsActive(apiKey)
		if !active {
			log.Printf("API key is not active")
			w.WriteHeader(http.StatusForbidden)
			e := ErrResponse{
				HTTPStatusCode: http.StatusForbidden,
				Message:        "API key is inactive.",
			}
			json.NewEncoder(w).Encode(e)
			return
		}
		//check if api key has the right rate limits.
		rateLimitAllowed := a.getRateLimits(r)
		if rateLimitAllowed == 0 {
			next.ServeHTTP(w, r) //does not exist in map so we assume it has no rate limit. not sure. i think we'd wanna put rate limits on every endpoint.
		}
		below, err := a.belowRateLimit(apiKey, rateLimitAllowed)
		if err != nil {
			log.Printf("API key validation error: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			e := ErrResponse{
				HTTPStatusCode: http.StatusUnauthorized,
				Message:        "Invalid API key",
			}
			json.NewEncoder(w).Encode(e)
			return
		}

		if !below {
			log.Printf("API key is beyond allowed rate limit")
			w.WriteHeader(http.StatusTooManyRequests)
			e := ErrResponse{
				HTTPStatusCode: http.StatusTooManyRequests,
				Message:        "Beyond the allowed rate limit",
			}
			json.NewEncoder(w).Encode(e)
			return
		}
		log.Printf("API rate middleware is successful: Key %s access %s %s", apiKey[:8]+"...", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (a *Api) ApiKeyAuthMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey, err := a.bearerToken(r)
		if err != nil {
			log.Printf("Token extraction Error: %v", err)
			w.WriteHeader((http.StatusUnauthorized))
			e := ErrResponse{
				HTTPStatusCode: http.StatusUnauthorized,
				Message:        "Invalid or missing authorization header",
			}
			json.NewEncoder(w).Encode(e)
			return
		}
		//check if api key exists and has permissions
		requiredPermission := a.getRequiredPermission(r)
		if requiredPermission == "" {
			next.ServeHTTP(w, r)
			return
		}
		hasPermission, err := a.ApiKeyHasPermissions(apiKey, requiredPermission)
		if err != nil {
			log.Printf("API key validation error: %v", err)
			w.WriteHeader(http.StatusForbidden)
			e := ErrResponse{
				HTTPStatusCode: http.StatusForbidden,
				Message:        "Invalid API key",
			}
			json.NewEncoder(w).Encode(e)
			return
		}
		if !hasPermission {
			log.Printf("Permission denied for key %s on %s %s:%v", apiKey[:8]+"...", r.Method, r.URL.Path, err)
			w.WriteHeader(http.StatusForbidden)
			e := ErrResponse{
				HTTPStatusCode: http.StatusForbidden,
				Message:        fmt.Sprintf("insufficient permissions for %s", requiredPermission),
			}
			json.NewEncoder(w).Encode(e)
			return
		}

		log.Printf("authentication is successful: Key %s access %s %s", apiKey[:8]+"...", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
func (a *Api) loadAPIKeys() {
	// For now, hardcode some test keys - replace with config file loading later
	a.APIConfig = APIkeysConfig{
		APIKeys: map[string]APIKeyInfo{
			"mk_manager_internal": {
				Permissions: []string{"admin:*"},
				Description: "Manager API key",
				CurrentRate: 0,
				RateLimits:  10000,
				IsActive:    true,
			},
			"mk_admin_xyz789": {
				Permissions: []string{"admin:*"},
				Description: "Admin access key",
				CurrentRate: 0,
				RateLimits:  5000,
				IsActive:    true,
				ExpiresAt:   "2030-09-09",
			},
			"mk_client_abc123": {
				Permissions: []string{"tasks:read", "tasks:write"},
				Description: "Client application key",
				CurrentRate: 0,
				RateLimits:  500,
				IsActive:    true,
				ExpiresAt:   "2029-04-02",
			},
			"mk_readonly_def456": {
				Permissions: []string{"tasks:read"},
				Description: "Read-only access key",
				CurrentRate: 0,
				RateLimits:  1000,
			},
		},
	}
	log.Printf("Loaded %d API keys", len(a.APIConfig.APIKeys))
}

func (a *Api) initRouter() {
	a.Router = chi.NewRouter()
	log.Printf("Initializing new router")
	a.loadAPIKeys()

	// Debug endpoints (no auth required)
	a.Router.Mount("/debug", http.DefaultServeMux)

	// API routes with authentication and rate limiting
	a.Router.Route("/tasks", func(r chi.Router) {
		r.Use(a.ApiKeyAuthMiddleware)
		r.Use(a.ApiRateLimitMiddleware)

		r.Post("/", a.StartTaskHandler)
		r.Get("/", a.GetTasksHandler)
		r.Route("/{taskID}", func(r chi.Router) {
			r.Delete("/", a.StopTaskHandler)
		})
	})

	// Serve static web UI files (no auth required) - MUST BE LAST as it catches all remaining routes
	fileServer := http.FileServer(http.Dir("./web"))
	a.Router.Handle("/*", fileServer)
}

func (a *Api) Start() {
	a.initRouter()
	// Create a server with various timeout settings
	addr := fmt.Sprintf("%s:%d", a.Address, a.Port)
	server := &http.Server{
		Addr: addr,
		// Maximum duration for reading the entire request
		ReadTimeout: 5 * time.Second,
		// Maximum duration for writing the response
		WriteTimeout: 10 * time.Second,
		// Maximum duration for reading the request headers
		ReadHeaderTimeout: 2 * time.Second,
		// Maximum amount of time to wait for the next request when keep-alives are enabled
		IdleTimeout: 120 * time.Second,
		// Handler to use for incoming requests
		Handler: a.Router,
	}

	log.Printf("Starting server on address:port, %s", addr)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
