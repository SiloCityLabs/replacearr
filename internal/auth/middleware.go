package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

type contextKey string

const (
	userContextKey    contextKey = "replacearr_user"
	apiKeyContextKey  contextKey = "replacearr_apikey"
	sessionContextKey contextKey = "replacearr_session"
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

func SessionFromContext(ctx context.Context) *domain.AuthSession {
	if ctx == nil {
		return nil
	}
	sess, _ := ctx.Value(sessionContextKey).(*domain.AuthSession)
	return sess
}

func (s *Service) Middleware(requiredScope string, requiredPerm string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			user    *domain.User
			apiKey  *domain.APIKey
			session *domain.AuthSession
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
				if u, sess, err := s.ValidateToken(token); err == nil {
					user = u
					session = sess
				} else if k, u, err := s.ValidateAPIKey(token, requiredScope); err == nil {
					apiKey = k
					user = u
				}
			}
		} else if cookie, err := r.Cookie("replacearr_token"); err == nil && strings.TrimSpace(cookie.Value) != "" {
			// 4. replacearr_token cookie
			if u, sess, err := s.ValidateToken(cookie.Value); err == nil {
				user = u
				session = sess
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
		if session != nil {
			ctx = context.WithValue(ctx, sessionContextKey, session)
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

func extractBearerOrCookieToken(r *http.Request) string {
	if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	if cookie, err := r.Cookie("replacearr_token"); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return xff
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
