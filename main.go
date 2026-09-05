package main

import (
	"fmt"
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

// Google Search Console verification handler
func googleVerifyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprint(
		w,
		"google-site-verification: google418121864bb557bd.html",
	)
}

// Proxy HTTP thông thường
func reverseProxy(target string) http.HandlerFunc {
	return corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		log.Printf(
			"🔄 HTTP Proxy: %s %s -> %s",
			r.Method,
			r.URL.Path,
			target,
		)

		targetURL, err := url.Parse(target)
		if err != nil {
			http.Error(
				w,
				"Bad target URL",
				http.StatusInternalServerError,
			)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director

		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			// /stock/xxx -> /xxx
			if strings.HasPrefix(req.URL.Path, "/stock/") {
				req.URL.Path = strings.TrimPrefix(
					req.URL.Path,
					"/stock",
				)

				log.Printf(
					"🔀 Path rewritten: %s",
					req.URL.Path,
				)

			} else if strings.HasPrefix(
				req.URL.Path,
				"/service-b/",
			) {

				// /service-b/xxx -> /xxx
				req.URL.Path = strings.TrimPrefix(
					req.URL.Path,
					"/service-b",
				)

				log.Printf(
					"🔀 Path rewritten: %s",
					req.URL.Path,
				)
			}

			// /auth/* KHÔNG rewrite
			//
			// /auth/register
			// ->
			// http://localhost:8003/auth/register
			//
			// /auth/login
			// ->
			// http://localhost:8003/auth/login
			//
			// /auth/verify-email
			// ->
			// http://localhost:8003/auth/verify-email
		}

		proxy.ErrorHandler = func(
			w http.ResponseWriter,
			r *http.Request,
			err error,
		) {
			log.Printf(
				"❌ HTTP Proxy error: %v",
				err,
			)

			w.Header().Set(
				"Access-Control-Allow-Origin",
				"*",
			)

			http.Error(
				w,
				"Backend service unavailable",
				http.StatusBadGateway,
			)
		}

		proxy.ServeHTTP(w, r)
	})
}

// WebSocket proxy
func websocketProxy(backendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf(
			"🔄 WS Proxy: %s %s -> %s",
			r.Method,
			r.URL.Path,
			backendURL,
		)

		targetURL, err := url.Parse(backendURL)

		if err != nil {
			http.Error(
				w,
				"Bad WebSocket target URL",
				http.StatusInternalServerError,
			)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(
			targetURL,
		)

		originalDirector := proxy.Director

		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			if strings.HasPrefix(
				req.URL.Path,
				"/ws2",
			) {
				// /ws2 -> backend /ws
				req.URL.Path = "/ws"

				log.Printf(
					"🔀 WS Path rewritten: %s",
					req.URL.Path,
				)

			} else if strings.HasPrefix(
				req.URL.Path,
				"/ws",
			) {
				// /ws stays /ws
				req.URL.Path = "/ws"

				log.Printf(
					"🔀 WS Path: %s",
					req.URL.Path,
				)
			}
		}

		proxy.ErrorHandler = func(
			w http.ResponseWriter,
			r *http.Request,
			err error,
		) {
			log.Printf(
				"❌ WebSocket proxy error: %v",
				err,
			)

			http.Error(
				w,
				"WebSocket backend unavailable",
				http.StatusBadGateway,
			)
		}

		proxy.ServeHTTP(w, r)
	}
}

// Health check
func healthCheck(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set(
		"Access-Control-Allow-Origin",
		"*",
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(
		http.StatusOK,
	)

	_, _ = w.Write(
		[]byte(
			`{"status":"healthy","message":"API Gateway is running"}`,
		),
	)
}

// WebSocket handler validation
func createWSHandler(
	backendURL string,
) http.HandlerFunc {

	wsProxy := websocketProxy(
		backendURL,
	)

	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		connection := strings.ToLower(
			r.Header.Get("Connection"),
		)

		upgrade := strings.ToLower(
			r.Header.Get("Upgrade"),
		)

		if strings.Contains(
			connection,
			"upgrade",
		) &&
			upgrade == "websocket" {

			wsProxy(w, r)

		} else {

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			http.Error(
				w,
				`{"error":"WebSocket upgrade required"}`,
				http.StatusBadRequest,
			)
		}
	}
}

// Proxy có rewrite prefix
func reverseProxyRewrite(
	target string,
	fromPrefix string,
	toPrefix string,
) http.HandlerFunc {

	return corsMiddleware(
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			log.Printf(
				"🔄 HTTP Proxy: %s %s -> %s",
				r.Method,
				r.URL.Path,
				target,
			)

			targetURL, err := url.Parse(target)

			if err != nil {
				http.Error(
					w,
					"Bad target URL",
					http.StatusInternalServerError,
				)
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(
				targetURL,
			)

			orig := proxy.Director

			proxy.Director = func(
				req *http.Request,
			) {
				orig(req)

				// /odoo
				// ->
				// /proxy
				//
				// /odoo/xxx
				// ->
				// /proxy/xxx

				if strings.HasPrefix(
					req.URL.Path,
					fromPrefix,
				) {

					oldPath := req.URL.Path

					req.URL.Path = strings.Replace(
						req.URL.Path,
						fromPrefix,
						toPrefix,
						1,
					)

					log.Printf(
						"🔀 Path rewritten: %s -> %s",
						oldPath,
						req.URL.Path,
					)
				}
			}

			proxy.ErrorHandler = func(
				w http.ResponseWriter,
				r *http.Request,
				err error,
			) {
				log.Printf(
					"❌ HTTP Proxy error: %v",
					err,
				)

				w.Header().Set(
					"Access-Control-Allow-Origin",
					"*",
				)

				http.Error(
					w,
					"Backend service unavailable",
					http.StatusBadGateway,
				)
			}

			proxy.ServeHTTP(
				w,
				r,
			)
		},
	)
}

func main() {

	// ========================================================
	// HEALTH
	// ========================================================

	http.HandleFunc(
		"/health",
		corsMiddleware(
			healthCheck,
		),
	)

	// ========================================================
	// GOOGLE VERIFY
	// ========================================================

	http.HandleFunc(
		"/google418121864bb557bd.html",
		googleVerifyHandler,
	)

	// ========================================================
	// STOCK SERVICE
	//
	// http://domain:8080/stock/xxx
	//
	// ->
	//
	// http://localhost:8001/xxx
	// ========================================================

	http.HandleFunc(
		"/stock/",
		reverseProxy(
			"http://localhost:8001",
		),
	)

	// ========================================================
	// SERVICE B
	//
	// http://domain:8080/service-b/xxx
	//
	// ->
	//
	// http://localhost:8002/xxx
	// ========================================================

	http.HandleFunc(
		"/service-b/",
		reverseProxy(
			"http://localhost:8002",
		),
	)

	// ========================================================
	// AUTH SERVICE
	//
	// Auth service chạy port 8003
	//
	// Gateway giữ nguyên /auth path
	//
	// POST /auth/register
	// ->
	// http://localhost:8003/auth/register
	//
	// POST /auth/verify-email
	// ->
	// http://localhost:8003/auth/verify-email
	//
	// POST /auth/login
	// ->
	// http://localhost:8003/auth/login
	// ========================================================

	http.HandleFunc(
		"/auth/",
		reverseProxy(
			"http://localhost:8003",
		),
	)

	// ========================================================
	// ODOO
	//
	// /odoo
	// ->
	// localhost:8000/proxy
	//
	// /odoo/xxx
	// ->
	// localhost:8000/proxy/xxx
	// ========================================================

	http.HandleFunc(
		"/odoo",
		reverseProxyRewrite(
			"http://localhost:8000",
			"/odoo",
			"/proxy",
		),
	)

	http.HandleFunc(
		"/odoo/",
		reverseProxyRewrite(
			"http://localhost:8000",
			"/odoo",
			"/proxy",
		),
	)

	// ========================================================
	// WEBSOCKET
	// ========================================================

	wsHandler9999 := createWSHandler(
		"http://localhost:9999",
	)

	wsHandler9998 := createWSHandler(
		"http://localhost:9998",
	)

	// /ws -> localhost:9999/ws

	http.HandleFunc(
		"/ws",
		wsHandler9999,
	)

	http.HandleFunc(
		"/ws/",
		wsHandler9999,
	)

	// /ws2 -> localhost:9998/ws

	http.HandleFunc(
		"/ws2",
		wsHandler9998,
	)

	http.HandleFunc(
		"/ws2/",
		wsHandler9998,
	)

	// ========================================================
	// LOG STARTUP
	// ========================================================

	log.Println(
		"🚀 API Gateway starting on http://0.0.0.0:8080",
	)

	log.Println(
		"📊 Routes configured:",
	)

	log.Println(
		"   🏥 Health:",
	)

	log.Println(
		"      GET http://localhost:8080/health",
	)

	log.Println(
		"   🔐 Auth:",
	)

	log.Println(
		"      POST http://localhost:8080/auth/register",
	)

	log.Println(
		"      POST http://localhost:8080/auth/verify-email",
	)

	log.Println(
		"      POST http://localhost:8080/auth/login",
	)

	log.Println(
		"      -> http://localhost:8003/auth/*",
	)

	log.Println(
		"   🌐 Stock:",
	)

	log.Println(
		"      http://localhost:8080/stock/*",
	)

	log.Println(
		"      -> http://localhost:8001/*",
	)

	log.Println(
		"   🌐 Service B:",
	)

	log.Println(
		"      http://localhost:8080/service-b/*",
	)

	log.Println(
		"      -> http://localhost:8002/*",
	)

	log.Println(
		"   🌐 Odoo:",
	)

	log.Println(
		"      http://localhost:8080/odoo/*",
	)

	log.Println(
		"      -> http://localhost:8000/proxy/*",
	)

	log.Println(
		"   📡 WebSocket:",
	)

	log.Println(
		"      ws://localhost:8080/ws",
	)

	log.Println(
		"      -> http://localhost:9999/ws",
	)

	log.Println(
		"      ws://localhost:8080/ws2",
	)

	log.Println(
		"      -> http://localhost:9998/ws",
	)

	log.Println(
		"🔓 CORS enabled for all origins",
	)

	// ========================================================
	// START SERVER
	// ========================================================

	log.Fatal(
		http.ListenAndServe(
			"0.0.0.0:8080",
			nil,
		),
	)
}
