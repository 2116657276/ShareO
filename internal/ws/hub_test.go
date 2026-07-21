package ws

import (
	"testing"
	"time"
)

func TestHubRegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	t.Cleanup(hub.Stop)
	client := &Client{hub: hub, userID: 42, send: make(chan []byte, 1)}
	hub.Register(client)
	eventually(t, func() bool { return hub.IsOnline(42) })
	hub.Unregister(client)
	if hub.IsOnline(42) {
		t.Fatal("user should be offline after the last connection unregisters")
	}
}

func TestHubKeepsUserOnlineUntilLastConnectionCloses(t *testing.T) {
	hub := NewHub()
	t.Cleanup(hub.Stop)
	a := &Client{hub: hub, userID: 7, send: make(chan []byte, 1)}
	b := &Client{hub: hub, userID: 7, send: make(chan []byte, 1)}
	hub.Register(a)
	hub.Register(b)
	eventually(t, func() bool {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		return len(hub.conns[7]) == 2
	})
	hub.Unregister(a)
	if !hub.IsOnline(7) {
		t.Fatal("second connection should keep user online")
	}
	hub.Unregister(b)
	if hub.IsOnline(7) {
		t.Fatal("user should be offline after both connections close")
	}
}

func TestHubDropsSlowConnection(t *testing.T) {
	hub := NewHub()
	t.Cleanup(hub.Stop)
	client := &Client{hub: hub, userID: 9, send: make(chan []byte, 1)}
	client.send <- []byte("already full")
	hub.Register(client)
	eventually(t, func() bool { return hub.IsOnline(9) })
	hub.SendToUsers([]int64{9}, map[string]string{"type": "probe"})
	eventually(t, func() bool { return !hub.IsOnline(9) })
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
