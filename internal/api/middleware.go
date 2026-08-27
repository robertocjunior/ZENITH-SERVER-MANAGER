package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

// SecurityHeadersMiddleware adds essential HTTP security headers to all responses.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self';")
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware validates Basic Auth or Bearer Token when configured.
func AuthMiddleware(cfg config.ServerConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow healthcheck without authentication for Docker and monitoring
		if r.URL.Path == "/api/v1/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		// If no authentication is configured, allow request
		if cfg.AuthPassword == "" && cfg.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 1. Check Bearer Token header
		authHeader := r.Header.Get("Authorization")
		if cfg.AuthToken != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AuthToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 2. Check Basic Auth
		if cfg.AuthPassword != "" {
			user, pass, ok := r.BasicAuth()
			if ok {
				userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.AuthUsername)) == 1
				passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.AuthPassword)) == 1
				if userMatch && passMatch {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Prompt for Basic Auth if unauthenticated
			w.Header().Set("WWW-Authenticate", `Basic realm="Zenith Server Manager"`)
			http.Error(w, "Unauthorized: Authentication required", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Unauthorized: Invalid or missing token", http.StatusUnauthorized)
	})
}

// LoggingMiddleware logs incoming HTTP requests.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[HTTP] %s %s from %s in %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}
