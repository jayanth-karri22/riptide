package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func wsServer(t *testing.T, handle func(*websocket.Conn)) string {
	t.Helper()
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func testCollector(url string, handle func(time.Time, []byte) error) *Collector {
	c := NewCollector(url, handle)
	c.MinBackoff = time.Millisecond
	c.MaxBackoff = 5 * time.Millisecond
	c.ResetAfter = time.Hour
	return c
}

func TestCollectorDeliversEveryFrame(t *testing.T) {
	url := wsServer(t, func(conn *websocket.Conn) {
		for _, m := range []string{`{"n":1}`, `{"n":2}`, `{"n":3}`} {
			if conn.WriteMessage(websocket.TextMessage, []byte(m)) != nil {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	})

	var mu sync.Mutex
	var got []string
	done := make(chan struct{})

	c := testCollector(url, func(_ time.Time, frame []byte) error {
		mu.Lock()
		got = append(got, string(frame))
		if len(got) == 3 {
			close(done)
		}
		mu.Unlock()
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out; got %v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 || got[0] != `{"n":1}` || got[2] != `{"n":3}` {
		t.Errorf("got %v", got)
	}
}

func TestCollectorReconnectsAfterDrop(t *testing.T) {
	var dials int64
	url := wsServer(t, func(conn *websocket.Conn) {
		n := atomic.AddInt64(&dials, 1)
		conn.WriteMessage(websocket.TextMessage, []byte(`{"conn":1}`))
		if n >= 3 {
			time.Sleep(200 * time.Millisecond)
		}
	})

	c := testCollector(url, func(time.Time, []byte) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt64(&dials) < 3 {
		select {
		case <-deadline:
			t.Fatalf("only %d connections; collector did not reconnect", atomic.LoadInt64(&dials))
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestCollectorAnswersServerPingWithSamePayload(t *testing.T) {
	pong := make(chan string, 1)
	url := wsServer(t, func(conn *websocket.Conn) {
		conn.SetPongHandler(func(payload string) error {
			select {
			case pong <- payload:
			default:
			}
			return nil
		})
		conn.WriteControl(websocket.PingMessage, []byte("keepalive-42"), time.Now().Add(time.Second))
		conn.SetReadDeadline(time.Now().Add(time.Second))
		conn.ReadMessage()
	})

	c := testCollector(url, func(time.Time, []byte) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	select {
	case got := <-pong:
		if got != "keepalive-42" {
			t.Errorf("pong payload = %q, want the ping's payload back", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no pong within 2s")
	}
}

func TestCollectorStopsOnContextCancel(t *testing.T) {
	url := wsServer(t, func(conn *websocket.Conn) { time.Sleep(time.Second) })

	c := testCollector(url, func(time.Time, []byte) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())

	errc := make(chan error, 1)
	go func() { errc <- c.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("Run returned %v, want nil on cancel", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestCollectorStopsWhenHandlerFails(t *testing.T) {
	url := wsServer(t, func(conn *websocket.Conn) {
		for {
			if conn.WriteMessage(websocket.TextMessage, []byte(`{}`)) != nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	})

	diskFull := errors.New("no space left on device")
	c := testCollector(url, func(time.Time, []byte) error { return diskFull })

	errc := make(chan error, 1)
	go func() { errc <- c.Run(context.Background()) }()

	select {
	case err := <-errc:
		if !errors.Is(err, diskFull) {
			t.Errorf("Run returned %v, want the handler error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run kept retrying through a handler failure")
	}
}

func TestCollectorBackoffGrowsAndCaps(t *testing.T) {
	s := time.Second
	for _, tc := range []struct {
		name     string
		min, max time.Duration
		want     []time.Duration
	}{
		{"cap on a doubling boundary", s, 8 * s, []time.Duration{0, s, 2 * s, 4 * s, 8 * s, 8 * s}},
		{"cap between doublings", s, 5 * s, []time.Duration{0, s, 2 * s, 4 * s, 5 * s, 5 * s}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testCollector("ws://unused", nil)
			c.MinBackoff, c.MaxBackoff = tc.min, tc.max
			for attempt, w := range tc.want {
				if got := c.backoff(attempt); got != w {
					t.Errorf("backoff(%d) = %v, want %v", attempt, got, w)
				}
			}
		})
	}
}

func TestCollectorBacksOffEvenWhenClosesAreClean(t *testing.T) {
	var dials int64
	url := wsServer(t, func(conn *websocket.Conn) {
		atomic.AddInt64(&dials, 1)
		conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
	})

	c := testCollector(url, func(time.Time, []byte) error { return nil })
	c.MinBackoff, c.MaxBackoff = 50*time.Millisecond, 50*time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	time.Sleep(400 * time.Millisecond)
	if n := atomic.LoadInt64(&dials); n > 12 {
		t.Errorf("%d connections in 400ms; a clean close must still back off", n)
	}
}
