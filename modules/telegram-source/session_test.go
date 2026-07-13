package main

import (
	"testing"
)

func TestSessionStoreActivePendingID(t *testing.T) {
	t.Parallel()

	store := newSessionStore(30)
	chatID := int64(123)

	if id, busy := store.activePendingID(chatID); busy || id != "" {
		t.Fatalf("idle chat: id=%q busy=%v", id, busy)
	}

	store.set(chatID, &session{
		Mode:             modeClassify,
		PendingRecordRef: "01JPENDING",
	})
	if id, busy := store.activePendingID(chatID); !busy || id != "01JPENDING" {
		t.Fatalf("classify session: id=%q busy=%v", id, busy)
	}

	store.clear(chatID)
	store.set(chatID, &session{Mode: modeFastPath})
	if id, busy := store.activePendingID(chatID); !busy || id != "" {
		t.Fatalf("fast path session: id=%q busy=%v", id, busy)
	}
}

func TestSessionStoreClear(t *testing.T) {
	t.Parallel()

	store := newSessionStore(30)
	chatID := int64(456)
	store.set(chatID, &session{Mode: modeClassify, PendingRecordRef: "01JTEST"})
	store.clear(chatID)
	if _, ok := store.get(chatID); ok {
		t.Fatal("session still present after clear")
	}
}
