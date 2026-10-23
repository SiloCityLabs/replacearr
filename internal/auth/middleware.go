package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

type contextKey string

const (
	userContextKey   contextKey = "replacearr_user"
	apiKeyContextKey contextKey = "replacearr_apikey"
)

func UserFromContext(ctx context.Context) *domain.User {
	if ctx == nil {
		return nil
	}
	u, _ := ctx.Value(userContextKey).(*domain.User)
	return u
}

func APIKeyFromContext(ctx context.Context) *domain.APIKey {
	if ctx == nil {
		return nil
	}
	k, _ := ctx.Value(apiKeyContextKey).(*domain.APIKey)
	return k
}

func (s *Service) Middleware(requiredScope string, requiredPerm string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			user   *domain.User
			apiKey *domain.APIKey
		)

		// 1. X-Api-Key header
		if hdrKey := strings.TrimSpace(r.Header.Get("X-Api-Key")); hdrKey != "" {
			if k, u, err := s.ValidateAPIKey(hdrKey, requiredScope); err == nil {
				apiKey = k
				user = u
			}
		} else if qKey := extractQueryAPIKey(r); qKey != "" {
			// 2. ?apikey= or ?apiKey= query parameter
			if k, u, err := s.ValidateAPIKey(qKey, requiredScope); err == nil {
				apiKey = k
				user = u
			}
		} else if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			// 3. Authorization: Bearer <token> (try JWT first, then API key)
			token := strings.TrimSpace(authHeader[7:])
			if token != "" {
				if u, err := s.ValidateToken(token); err == nil {
					user = u
				} else if k, u, err := s.ValidateAPIKey(token, requiredScope); err == nil {
					apiKey = k
					user = u
				}
			}
		} else if cookie, err := r.Cookie("replacearr_token"); err == nil && strings.TrimSpace(cookie.Value) != "" {
			// 4. replacearr_token cookie
			if u, err := s.ValidateToken(cookie.Value); err == nil {
				user = u
			}
		}

		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
			})
			return
		}

		if requiredPerm != "" && !s.HasPermission(user, apiKey, requiredPerm) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "forbidden: missing permission " + requiredPerm,
			})
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		if apiKey != nil {
			ctx = context.WithValue(ctx, apiKeyContextKey, apiKey)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) RequirePermission(perm string, next http.HandlerFunc) http.HandlerFunc {
	return s.Middleware("", perm, next).ServeHTTP
}

func extractQueryAPIKey(r *http.Request) string {
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("apikey")); v != "" {
		return v
	}
	if v := strings.TrimSpace(q.Get("apiKey")); v != "" {
		return v
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
