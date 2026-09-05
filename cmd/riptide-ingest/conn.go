package main

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultReadTimeout = 60 * time.Second
	defaultMinBackoff  = time.Second
	defaultMaxBackoff  = 30 * time.Second
	defaultResetAfter  = time.Minute
)

type Collector struct {
	URL         string
	Handle      func(time.Time, []byte) error
	Dialer      *websocket.Dialer
	ReadTimeout time.Duration
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
	ResetAfter  time.Duration
	OnConnect   func()
	OnDropped   func(error)
}

func NewCollector(url string, handle func(time.Time, []byte) error) *Collector {
	return &Collector{
		URL:         url,
		Handle:      handle,
		Dialer:      websocket.DefaultDialer,
		ReadTimeout: defaultReadTimeout,
		MinBackoff:  defaultMinBackoff,
		MaxBackoff:  defaultMaxBackoff,
		ResetAfter:  defaultResetAfter,
	}
}

func (c *Collector) Run(ctx context.Context) error {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return nil
		}
		start := time.Now()
		err := c.session(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if c.OnDropped != nil {
			c.OnDropped(err)
		}
		if handlerFailed(err) {
			return err
		}
		if time.Since(start) >= c.ResetAfter {
			attempt = 0
		}
		delay := c.backoff(attempt)
		attempt++

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
	}
}

func (c *Collector) session(ctx context.Context) error {
	conn, _, err := c.Dialer.DialContext(ctx, c.URL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()

	extend := func() { _ = conn.SetReadDeadline(time.Now().Add(c.ReadTimeout)) }
	extend()
	conn.SetPingHandler(func(payload string) error {
		extend()
		return conn.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(10*time.Second))
	})

	if c.OnConnect != nil {
		c.OnConnect()
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		extend()
		if err := c.Handle(time.Now(), msg); err != nil {
			return handlerError{err}
		}
	}
}

func (c *Collector) backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	d := c.MinBackoff
	for i := 1; i < attempt && d < c.MaxBackoff; i++ {
		d *= 2
	}
	if d > c.MaxBackoff {
		d = c.MaxBackoff
	}
	return d
}

type handlerError struct{ err error }

func (h handlerError) Error() string { return h.err.Error() }
func (h handlerError) Unwrap() error { return h.err }

func handlerFailed(err error) bool {
	_, ok := err.(handlerError)
	return ok
}
