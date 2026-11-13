package realtime

import (
	"testing"
	"time"
)

func TestHubBroadcast(t *testing.T) {
	hub := NewHub("")
	defer hub.Close()
	fast, slow := newClient(), newClient()
	if !hub.add(fast) || !hub.add(slow) {
		t.Fatal("registration failed")
	}
	receive := func(c *client) {
		t.Helper()
		select {
		case <-c.send:
		case <-time.After(time.Second):
			t.Fatal("broadcast missing")
		}
	}
	hub.queue <- []byte("first")
	receive(fast)
	receive(slow)
	for range clientBuffer + 1 {
		hub.queue <- []byte("next")
		receive(fast)
	}
	select {
	case <-slow.done:
	case <-time.After(time.Second):
		t.Fatal("slow client was not dropped")
	}
	leaving := newClient()
	if !hub.add(leaving) {
		t.Fatal("registration failed")
	}
	unregistered := make(chan struct{})
	go func() { hub.remove(leaving); close(unregistered) }()
	hub.queue <- []byte("after disconnect")
	receive(fast)
	<-unregistered
	select {
	case <-leaving.done:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not remove client")
	}
}
