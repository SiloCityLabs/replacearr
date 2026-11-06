package libraryimport

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultPageSize   = 100
	DefaultSessionTTL = 60 * time.Minute
)

// Session holds unmapped scan candidates for paginated matching/import.
type Session struct {
	ID        string
	MediaType string
	Root      string
	Items     []ScanItem
	CreatedAt time.Time
}

// SessionStore is an in-process TTL map of library-import scan sessions.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
}

// NewSessionStore creates a session store. ttl <= 0 uses DefaultSessionTTL.
func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &SessionStore{
		sessions: map[string]*Session{},
		ttl:      ttl,
	}
}

func (s *SessionStore) Create(mediaType, root string, items []ScanItem) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(time.Now())
	id := newSessionID()
	cp := make([]ScanItem, len(items))
	copy(cp, items)
	s.sessions[id] = &Session{
		ID:        id,
		MediaType: mediaType,
		Root:      root,
		Items:     cp,
		CreatedAt: time.Now().UTC(),
	}
	return id
}

func (s *SessionStore) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(time.Now())
	sess, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	return sess, true
}

// Page returns a copy of items for [offset, offset+limit) and the total count.
func (s *SessionStore) Page(id string, offset, limit int) ([]ScanItem, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(time.Now())
	sess, ok := s.sessions[id]
	if !ok {
		return nil, 0, fmt.Errorf("scan session not found or expired")
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = DefaultPageSize
	}
	total := len(sess.Items)
	if offset >= total {
		return []ScanItem{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	out := make([]ScanItem, end-offset)
	copy(out, sess.Items[offset:end])
	return out, total, nil
}

// RemovePaths drops imported paths from the session and returns the new total.
func (s *SessionStore) RemovePaths(id string, paths []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(time.Now())
	sess, ok := s.sessions[id]
	if !ok {
		return 0, fmt.Errorf("scan session not found or expired")
	}
	drop := map[string]bool{}
	for _, p := range paths {
		if p = filepath.Clean(p); p != "" {
			drop[p] = true
		}
	}
	kept := sess.Items[:0]
	for _, it := range sess.Items {
		if drop[filepath.Clean(it.Path)] {
			continue
		}
		kept = append(kept, it)
	}
	sess.Items = kept
	return len(sess.Items), nil
}

func (s *SessionStore) purgeExpiredLocked(now time.Time) {
	for id, sess := range s.sessions {
		if now.Sub(sess.CreatedAt) > s.ttl {
			delete(s.sessions, id)
		}
	}
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
