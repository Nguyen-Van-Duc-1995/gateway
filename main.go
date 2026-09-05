package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue to next handler
		next(w, r)
	}
}

// Proxy HTTP thông thường với CORS
func reverseProxy(target string) http.HandlerFunc {
	return corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔄 HTTP Proxy: %s %s -> %s", r.Method, r.URL.Path, target)

		targetURL, err := url.Parse(target)
		if err != nil {
			http.Error(w, "Bad target URL", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// Ghi đè Director để chỉnh path
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			// Xóa tiền tố "/stock" hoặc "/service-b"
			if strings.HasPrefix(req.URL.Path, "/stock/") {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, "/stock")
				log.Printf("🔀 Path rewritten: %s", req.URL.Path)
			} else if strings.HasPrefix(req.URL.Path, "/service-b/") {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, "/service-b")
				log.Printf("🔀 Path rewritten: %s", req.URL.Path)
			}

			// Không rewrite /auth
			// Ví dụ:
			// /auth/register -> /auth/register
			// /auth/login -> /auth/login
			// /auth/verify-email -> /auth/verify-email
		}

		// Custom error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("❌ HTTP Proxy error: %v", err)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			http.Error(w, "Backend service unavailable", http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	})
}

// ✅ WebSocket proxy sử dụng httputil.ReverseProxy
func websocketProxy(backendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔄 WS Proxy: %s %s -> %s", r.Method, r.URL.Path, backendURL)

		// Parse backend URL
		targetURL, err := url.Parse(backendURL)
		if err != nil {
			http.Error(w, "Bad WebSocket target URL", http.StatusInternalServerError)
			return
		}

		// Create reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// Modify the director to handle WebSocket path
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			// Rewrite paths for WebSocket
			if strings.HasPrefix(req.URL.Path, "/ws2") {
				// /ws2 -> /ws (port 9998)
				req.URL.Path = "/ws"
				log.Printf("🔀 WS Path rewritten: %s", req.URL.Path)
			} else if strings.HasPrefix(req.URL.Path, "/ws") {
				// /ws stays /ws (port 9999)
				req.URL.Path = "/ws"
				log.Printf("🔀 WS Path: %s", req.URL.Path)
			}
		}

		// Custom error handler for WebSocket
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("❌ WebSocket proxy error: %v", err)
			http.Error(w, "WebSocket backend unavailable", http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	}
}

// Health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`{
		"status": "healthy",
		"message": "API Gateway is running"
	}`))
}

// ✅ WebSocket route handler với validation
func createWSHandler(backendURL string) http.HandlerFunc {
	wsProxy := websocketProxy(backendURL)

	return func(w http.ResponseWriter, r *http.Request) {
		// Kiểm tra xem có phải WebSocket request không
		if strings.Contains(
			strings.ToLower(r.Header.Get("Connection")),
			"upgrade",
		) &&
			strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {

			wsProxy(w, r)

		} else {
			// Nếu không phải WebSocket, trả về error thân thiện
			w.Header().Set("Content-Type", "application/json")

			http.Error(
				w,
				`{"error": "WebSocket upgrade required"}`,
				http.StatusBadRequest,
			)
		}
	}
}

func main() {
	// ✅ Health check endpoint
	http.HandleFunc(
		"/health",
		corsMiddleware(healthCheck),
	)

	// ✅ HTTP reverse proxy with CORS
	http.HandleFunc(
		"/stock/",
		reverseProxy("http://localhost:8001"),
	)

	http.HandleFunc(
		"/service-b/",
		reverseProxy("http://localhost:8002"),
	)

	// ✅ Auth service
	//
	// Flutter:
	// POST http://localhost:8080/auth/register
	// POST http://localhost:8080/auth/verify-email
	// POST http://localhost:8080/auth/login
	//
	// Gateway sẽ forward nguyên path sang port 8003:
	// /auth/register       -> http://localhost:8003/auth/register
	// /auth/verify-email   -> http://localhost:8003/auth/verify-email
	// /auth/login          -> http://localhost:8003/auth/login
	http.HandleFunc(
		"/auth/",
		reverseProxy("http://localhost:8003"),
	)

	// ✅ WebSocket proxy handlers - SỬ DỤNG HTTP SCHEME
	wsHandler9999 := createWSHandler(
		"http://localhost:9999",
	)

	wsHandler9998 := createWSHandler(
		"http://localhost:9998",
	)

	// ✅ WebSocket routes
	http.HandleFunc(
		"/ws",
		wsHandler9999,
	)

	http.HandleFunc(
		"/ws/",
		wsHandler9999,
	)

	http.HandleFunc(
		"/ws2",
		wsHandler9998,
	)

	http.HandleFunc(
		"/ws2/",
		wsHandler9998,
	)

	// ✅ Logging thông tin khởi động
	log.Println("🚀 API Gateway starting on http://0.0.0.0:8080")
	log.Println("📊 Routes configured:")

	log.Println(
		"   📡 WebSocket: ws://localhost:8080/ws  -> http://localhost:9999/ws",
	)

	log.Println(
		"   📡 WebSocket: ws://localhost:8080/ws2 -> http://localhost:9998/ws",
	)

	log.Println(
		"   🌐 HTTP: http://localhost:8080/stock/* -> http://localhost:8001/*",
	)

	log.Println(
		"   🌐 HTTP: http://localhost:8080/service-b/* -> http://localhost:8002/*",
	)

	// ✅ Auth routes
	log.Println(
		"   🔐 AUTH: http://localhost:8080/auth/* -> http://localhost:8003/auth/*",
	)

	log.Println(
		"      POST /auth/register",
	)

	log.Println(
		"      POST /auth/verify-email",
	)

	log.Println(
		"      POST /auth/login",
	)

	log.Println(
		"   🏥 Health: http://localhost:8080/health",
	)

	log.Println(
		"🔐 CORS enabled for all origins",
	)

	// Bind to 0.0.0.0 để accept external connections
	log.Fatal(
		http.ListenAndServe(
			"0.0.0.0:8080",
			nil,
		),
	)
}
