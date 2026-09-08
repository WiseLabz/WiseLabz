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
