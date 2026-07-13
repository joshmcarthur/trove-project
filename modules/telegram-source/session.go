package main

import (
	"sync"
	"time"
)

type sessionMode int

const (
	modeClassify sessionMode = iota
	modeFastPath
)

type captureDraft struct {
	Time        string
	BlobRef     string
	Text        string
	CaptureJSON []byte
}

type session struct {
	Mode             sessionMode
	PendingRecordRef string
	TargetType       string
	AwaitingContent  bool
	FieldIndex       int
	Collected        map[string]string
	Draft            *captureDraft
	UpdatedAt        time.Time
}

type sessionStore struct {
	mu       sync.Mutex
	sessions map[int64]*session
	ttl      time.Duration
}

func newSessionStore(ttlMin int) *sessionStore {
	return &sessionStore{
		sessions: make(map[int64]*session),
		ttl:      time.Duration(ttlMin) * time.Minute,
	}
}

func (s *sessionStore) get(chatID int64) (*session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked()
	sess, ok := s.sessions[chatID]
	if !ok {
		return nil, false
	}
	return sess, true
}

func (s *sessionStore) set(chatID int64, sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked()
	sess.UpdatedAt = time.Now()
	s.sessions[chatID] = sess
}

func (s *sessionStore) clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, chatID)
}

func (s *sessionStore) activePendingID(chatID int64) (string, bool) {
	sess, ok := s.get(chatID)
	if !ok || sess == nil {
		return "", false
	}
	if sess.Mode == modeClassify && sess.PendingRecordRef != "" {
		return sess.PendingRecordRef, true
	}
	if sess.Mode == modeFastPath {
		return "", true
	}
	return "", false
}

func (s *sessionStore) expireLocked() {
	if s.ttl <= 0 {
		return
	}
	cutoff := time.Now().Add(-s.ttl)
	for chatID, sess := range s.sessions {
		if sess.UpdatedAt.Before(cutoff) {
			delete(s.sessions, chatID)
		}
	}
}
