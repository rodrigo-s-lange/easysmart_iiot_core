package middleware

import (
	"net/http"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods string
	AllowedHeaders string
}

func NewCORSConfig(originsCSV, methods, headers string) *CORSConfig {
	var origins []string
	for _, o := range strings.Split(originsCSV, ",") {
		if v := strings.TrimSpace(o); v != "" {
			origins = append(origins, v)
		}
	}
	return &CORSConfig{
		AllowedOrigins: origins,
		AllowedMethods: methods,
		AllowedHeaders: headers,
	}
}

func (c *CORSConfig) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || len(c.AllowedOrigins) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		allowOrigin := ""
		for _, o := range c.AllowedOrigins {
			if o == "*" {
				allowOrigin = "*"
				break
			}
			if o == origin {
				allowOrigin = origin
				break
			}
		}

		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", c.AllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", c.AllowedHeaders)
			if allowOrigin != "*" {
				w.Header().Add("Vary", "Origin")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
