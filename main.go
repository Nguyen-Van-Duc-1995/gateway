package main

import (
	"fmt" // 👈 THÊM VÀO ĐÂY
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// ✅ Google Search Console verification handler (THÊM MỚI)
func googleVerifyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// 👇 THAY nguyên văn nội dung bên trong file google418121864bb557bd.html
	fmt.Fprint(w, "google-site-verification: google418121864bb557bd.html")
}

// Proxy HTTP thông thường
func reverseProxy(target string) http.HandlerFunc {
	return corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔄 HTTP Proxy: %s %s -> %s", r.Method, r.URL.Path, target)

		targetURL, err := url.Parse(target)
		if err != nil {
			http.Error(w, "Bad target URL", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			if strings.HasPrefix(req.URL.Path, "/stock/") {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, "/stock")
			} else if strings.HasPrefix(req.URL.Path, "/service-b/") {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, "/service-b")
			}
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("❌ HTTP Proxy error: %v", err)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			http.Error(w, "Backend service unavailable", http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	})
}

func websocketProxy(backendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔄 WS Proxy: %s %s -> %s", r.Method, r.URL.Path, backendURL)

		targetURL, err := url.Parse(backendURL)
		if err != nil {
			http.Error(w, "Bad WebSocket target URL", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		originalDirector := proxy.Director

		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			if strings.HasPrefix(req.URL.Path, "/ws2") {
				req.URL.Path = "/ws"
			} else if strings.HasPrefix(req.URL.Path, "/ws") {
				req.URL.Path = "/ws"
			}
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("❌ WebSocket proxy error: %v", err)
			http.Error(w, "WebSocket backend unavailable", http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	}
}

// Health check
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy", "message": "API Gateway is running"}`))
}

func createWSHandler(backendURL string) http.HandlerFunc {
	wsProxy := websocketProxy(backendURL)
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
			strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {

			wsProxy(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "WebSocket upgrade required"}`, http.StatusBadRequest)
		}
	}
}

func main() {
	// Health
	http.HandleFunc("/health", corsMiddleware(healthCheck))

	// ✅ Google verification route (THÊM 1 DÒNG DUY NHẤT)
	http.HandleFunc("/google418121864bb557bd.html", googleVerifyHandler)

	// Proxies
	http.HandleFunc("/stock/", reverseProxy("http://localhost:8001"))
	http.HandleFunc("/service-b/", reverseProxy("http://localhost:8002"))

	wsHandler9999 := createWSHandler("http://localhost:9999")
	wsHandler9998 := createWSHandler("http://localhost:9998")

	http.HandleFunc("/ws", wsHandler9999)
	http.HandleFunc("/ws/", wsHandler9999)
	http.HandleFunc("/ws2", wsHandler9998)
	http.HandleFunc("/ws2/", wsHandler9998)

	log.Println("🚀 API Gateway starting on http://0.0.0.0:8080")

	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
