package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// APIKey requires a matching key on every request except those whose path is
// in the allow list (e.g. health check, docs). The key may be supplied as the
// `key` query param or the `X-API-Key` header. If key is empty, auth is off.
func APIKey(key string, allow ...string) Middleware {
	allowSet := make(map[string]struct{}, len(allow))
	for _, p := range allow {
		allowSet[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := allowSet[r.URL.Path]; ok || isPrefixAllowed(r.URL.Path, allow) {
				next.ServeHTTP(w, r)
				return
			}
			got := r.URL.Query().Get("key")
			if got == "" {
				got = r.Header.Get("X-API-Key")
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(key)) != 1 {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isPrefixAllowed(path string, allow []string) bool {
	for _, p := range allow {
		if strings.HasSuffix(p, "/") && strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
