package main

import (
	"fmt"
	"strings"
	"sync"
)

type subscriptionState struct {
	mu         sync.RWMutex
	connected  bool
	subscribed map[string]bool
	wantTopics []string
}

func newSubscriptionState(topics []string) *subscriptionState {
	return &subscriptionState{
		subscribed: make(map[string]bool, len(topics)),
		wantTopics: append([]string(nil), topics...),
	}
}

func (s *subscriptionState) setConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = connected
	if !connected {
		for topic := range s.subscribed {
			s.subscribed[topic] = false
		}
	}
}

func (s *subscriptionState) setSubscribed(topic string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribed[topic] = ok
}

func (s *subscriptionState) healthMessage() (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected {
		return false, "mqtt client disconnected"
	}

	parts := make([]string, 0, len(s.wantTopics))
	allOK := true
	for _, topic := range s.wantTopics {
		ok := s.subscribed[topic]
		if !ok {
			allOK = false
		}
		status := "pending"
		if ok {
			status = "subscribed"
		}
		parts = append(parts, fmt.Sprintf("%s=%s", topic, status))
	}

	return allOK, "connected; " + strings.Join(parts, ", ")
}
