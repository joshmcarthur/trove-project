package journal

import (
	"context"
	"fmt"
)

// Watch returns a coalesced wakeup channel signaled after each Append.
// Payloads are not delivered; callers pull events from the journal separately.
func (s *Store) Watch(ctx context.Context) (<-chan struct{}, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("journal: watch: %w", err)
	}

	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.appendWatchers = append(s.appendWatchers, ch)
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.removeWatcher(ch)
	}()

	return ch, nil
}

func (s *Store) removeWatcher(ch chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, watcher := range s.appendWatchers {
		if watcher == ch {
			s.appendWatchers = append(s.appendWatchers[:i], s.appendWatchers[i+1:]...)
			close(ch)
			return
		}
	}
}

func (s *Store) signalWatchers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.appendWatchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
