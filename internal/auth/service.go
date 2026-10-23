package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/replacearr/replacearr/internal/config"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const MasterKeyName = "System Master API Key"

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInactiveUser       = errors.New("user account is inactive")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrInvalidAPIKey      = errors.New("invalid api key")
	ErrScopeMismatch      = errors.New("api key scope mismatch")
)

type Claims struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	store        *db.Store
	cfg          *config.Config
	mu           sync.RWMutex
	masterAPIKey string
}

func NewService(store *db.Store, cfg *config.Config) (*Service, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}
	if cfg == nil {
		cfg = config.Load()
	}
	if cfg.AdminUser == "" {
		cfg.AdminUser = "admin"
	}
	if cfg.AdminPass == "" {
		cfg.AdminPass = "replacearr"
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "replacearr-default-jwt-secret-key-32b"
	}

	s := &Service{
		store: store,
		cfg:   cfg,
	}
	if err := s.EnsureBootstrap(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) EnsureBootstrap() error {
	users, err := s.store.ListUsers()
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	keyFilePath := ""
	if s.cfg.ConfigDir != "" {
		_ = os.MkdirAll(s.cfg.ConfigDir, 0o755)
		keyFilePath = filepath.Join(s.cfg.ConfigDir, "api_key.txt")
	}

	if len(users) == 0 {
		adminRole, err := s.store.GetRoleByName("admin")
		if err != nil {
			return fmt.Errorf("get admin role: %w", err)
		}

		passHash, err := s.HashPassword(s.cfg.AdminPass)
		if err != nil {
			return fmt.Errorf("hash admin password: %w", err)
		}

		adminUser, err := s.store.CreateUser(s.cfg.AdminUser, "admin@replacearr.local", passHash, adminRole.ID)
		if err != nil {
			return fmt.Errorf("create bootstrap admin user: %w", err)
		}

		rawKey := strings.TrimSpace(s.cfg.DefaultAPIKey)
		if rawKey == "" && keyFilePath != "" {
			if b, err := os.ReadFile(keyFilePath); err == nil {
				candidate := strings.TrimSpace(string(b))
				if len(candidate) >= 16 {
					rawKey = candidate
				}
			}
		}
		if rawKey == "" {
			generated, err := generateRandomHex(16) // 32 hex chars
			if err != nil {
				return fmt.Errorf("generate master api key: %w", err)
			}
			rawKey = generated
		}

		if keyFilePath != "" {
			_ = os.WriteFile(keyFilePath, []byte(rawKey), 0o600)
		}

		keyHash := hashKey(rawKey)
		keyPrefix := prefixKey(rawKey)
		if _, err := s.store.CreateAPIKey(MasterKeyName, keyHash, keyPrefix, rawKey, adminUser.ID, "all", domain.AllPermissions); err != nil {
			return fmt.Errorf("create master api key: %w", err)
		}

		s.mu.Lock()
		s.masterAPIKey = rawKey
		s.mu.Unlock()
		return nil
	}

	// Users already exist; load existing master API key from DB or disk
	keys, err := s.store.ListAPIKeys()
	if err == nil {
		for _, k := range keys {
			if k.Name == MasterKeyName && k.RawKey != "" {
				s.mu.Lock()
				s.masterAPIKey = k.RawKey
				s.mu.Unlock()
				if keyFilePath != "" {
					_ = os.WriteFile(keyFilePath, []byte(k.RawKey), 0o600)
				}
				return nil
			}
		}
		if len(keys) > 0 && keys[0].RawKey != "" {
			s.mu.Lock()
			s.masterAPIKey = keys[0].RawKey
			s.mu.Unlock()
			return nil
		}
	}

	if keyFilePath != "" {
		if b, err := os.ReadFile(keyFilePath); err == nil {
			s.mu.Lock()
			s.masterAPIKey = strings.TrimSpace(string(b))
			s.mu.Unlock()
		}
	}

	return nil
}

func (s *Service) HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Service) VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func (s *Service) Authenticate(username, password string) (*domain.User, string, error) {
	u, err := s.store.GetUserByUsername(strings.TrimSpace(username))
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if !u.IsActive {
		return nil, "", ErrInactiveUser
	}
	if !s.VerifyPassword(u.PasswordHash, password) {
		return nil, "", ErrInvalidCredentials
	}

	s.store.TouchUserLogin(u.ID)
	now := time.Now().UTC()
	u.LastLoginAt = &now

	claims := Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.RoleName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", u.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			Issuer:    "replacearr",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, "", fmt.Errorf("sign jwt: %w", err)
	}

	return u, signed, nil
}

func (s *Service) ValidateToken(tokenStr string) (*domain.User, error) {
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return nil, ErrInvalidToken
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid || claims.UserID <= 0 {
		return nil, ErrInvalidToken
	}

	u, err := s.store.GetUserByID(claims.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !u.IsActive {
		return nil, ErrInactiveUser
	}
	return u, nil
}

func (s *Service) GenerateAPIKey(name string, userID int64, scope string, permissions []string) (*domain.APIKey, error) {
	if strings.TrimSpace(name) == "" {
		name = "API Key"
	}
	if scope == "" {
		scope = "all"
	}
	if permissions == nil {
		permissions = []string{}
	}

	rawKey, err := generateRandomHex(16) // 32 hex chars
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	keyHash := hashKey(rawKey)
	keyPrefix := prefixKey(rawKey)

	apiKey, err := s.store.CreateAPIKey(name, keyHash, keyPrefix, rawKey, userID, scope, permissions)
	if err != nil {
		return nil, err
	}
	apiKey.RawKey = rawKey
	return apiKey, nil
}

func (s *Service) ValidateAPIKey(rawKey string, requiredScope string) (*domain.APIKey, *domain.User, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return nil, nil, ErrInvalidAPIKey
	}

	keyHash := hashKey(rawKey)
	apiKey, user, err := s.store.GetAPIKeyByHash(keyHash)
	if err != nil {
		return nil, nil, ErrInvalidAPIKey
	}
	if user == nil || !user.IsActive {
		return nil, nil, ErrInactiveUser
	}

	if !(apiKey.Scope == "all" || apiKey.Scope == requiredScope || requiredScope == "") {
		return nil, nil, ErrScopeMismatch
	}

	return apiKey, user, nil
}

func (s *Service) GetMasterAPIKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.masterAPIKey
}

func (s *Service) RotateMasterAPIKey() (string, error) {
	key, err := s.store.GetAPIKeyByName(MasterKeyName)
	if err != nil {
		return "", err
	}
	rawKey, err := generateRandomHex(16)
	if err != nil {
		return "", fmt.Errorf("generate master api key: %w", err)
	}
	if err := s.store.ReplaceAPIKeySecret(key.ID, hashKey(rawKey), prefixKey(rawKey), rawKey); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.masterAPIKey = rawKey
	s.mu.Unlock()
	if s.cfg != nil && s.cfg.ConfigDir != "" {
		path := filepath.Join(s.cfg.ConfigDir, "api_key.txt")
		if err := os.WriteFile(path, []byte(rawKey), 0o600); err != nil {
			return "", err
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return "", err
		}
	}
	return rawKey, nil
}

func (s *Service) HasPermission(user *domain.User, apiKey *domain.APIKey, perm string) bool {
	if perm == "" {
		return true
	}
	if user == nil || !user.IsActive {
		return false
	}

	if !sliceHasPerm(user.Permissions, perm) {
		return false
	}

	if apiKey != nil && len(apiKey.Permissions) > 0 {
		if !sliceHasPerm(apiKey.Permissions, perm) {
			return false
		}
	}

	return true
}

func sliceHasPerm(perms []string, target string) bool {
	for _, p := range perms {
		if p == domain.PermAdminAll || p == target {
			return true
		}
	}
	if target == domain.PermAdminAll && len(perms) >= len(domain.AllPermissions) {
		return true
	}
	return false
}

func generateRandomHex(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func prefixKey(raw string) string {
	if len(raw) >= 8 {
		return raw[:8]
	}
	return raw
}
