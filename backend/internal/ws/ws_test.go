package ws

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHubConfiguredOrigin(t *testing.T) {
	hub := NewHub("https://app.example.com")
	allowed := httptest.NewRequest("GET", "/api/ws", nil)
	allowed.Header.Set("Origin", "https://app.example.com")
	if !hub.upgrader.CheckOrigin(allowed) {
		t.Fatal("configured origin was rejected")
	}
	rejected := httptest.NewRequest("GET", "/api/ws", nil)
	rejected.Header.Set("Origin", "https://evil.example.com")
	if hub.upgrader.CheckOrigin(rejected) {
		t.Fatal("cross-origin request was accepted")
	}
}

func TestHubBroadcastRouting(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	first := &Client{hub: hub, send: make(chan []byte, 1), userID: "first"}
	second := &Client{hub: hub, send: make(chan []byte, 1), userID: "second"}
	hub.register <- first
	hub.register <- second
	t.Cleanup(func() {
		hub.unregister <- first
		hub.unregister <- second
	})

	deadline := time.Now().Add(time.Second)
	for hub.ClientCount() != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := hub.ClientCount(); got != 2 {
		t.Fatalf("ClientCount() = %d, want 2", got)
	}
	hub.Broadcast(EventSyncComplete, map[string]string{"job": "all"})
	assertEnvelope(t, <-first.send, EventSyncComplete, "all")
	assertEnvelope(t, <-second.send, EventSyncComplete, "all")

	hub.BroadcastToUser("first", EventSystemNotice, map[string]string{"job": "one"})
	assertEnvelope(t, <-first.send, EventSystemNotice, "one")
	select {
	case data := <-second.send:
		t.Fatalf("second user received %s", data)
	case <-time.After(25 * time.Millisecond):
	}
}

func assertEnvelope(t *testing.T, data []byte, wantType, wantJob string) {
	t.Helper()
	var got struct {
		Type    string            `json:"type"`
		Payload map[string]string `json:"payload"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got.Type != wantType || got.Payload["job"] != wantJob {
		t.Fatalf("envelope = %+v", got)
	}
}

func TestDocLockEventBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{hub: hub, send: make(chan []byte, 3), userID: "test"}
	hub.register <- client
	t.Cleanup(func() {
		hub.unregister <- client
	})

	deadline := time.Now().Add(time.Second)
	for hub.ClientCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount() = %d, want 1", got)
	}

	hub.Broadcast(EventDocLockAcquired, map[string]string{"docId": "doc-1", "userId": "user-1"})
	hub.Broadcast(EventDocLockReleased, map[string]string{"docId": "doc-1", "userId": "user-1"})
	hub.Broadcast(EventDocLockExpired, map[string]string{"docId": "doc-1", "userId": "user-1"})

	var env struct {
		Type string `json:"type"`
	}

	data := <-client.send
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal event 1: %v", err)
	}
	if env.Type != EventDocLockAcquired {
		t.Fatalf("event 1: got type %q, want %q", env.Type, EventDocLockAcquired)
	}

	data = <-client.send
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal event 2: %v", err)
	}
	if env.Type != EventDocLockReleased {
		t.Fatalf("event 2: got type %q, want %q", env.Type, EventDocLockReleased)
	}

	data = <-client.send
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal event 3: %v", err)
	}
	if env.Type != EventDocLockExpired {
		t.Fatalf("event 3: got type %q, want %q", env.Type, EventDocLockExpired)
	}
}
