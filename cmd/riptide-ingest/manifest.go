package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

const unknownStream = "?"

type streamStats struct {
	Stream      string `json:"stream"`
	Count       int64  `json:"count"`
	FirstEvent  int64  `json:"first_event_ms"`
	LastEvent   int64  `json:"last_event_ms"`
	EventSource string `json:"event_time_source"`
	HasID       bool   `json:"has_id"`
	FirstID     int64  `json:"first_id"`
	LastID      int64  `json:"last_id"`
}

type Manifest struct {
	venue   string
	hour    string
	frames  int64
	streams map[string]*streamStats
}

type manifestDoc struct {
	Venue   string         `json:"venue"`
	Hour    string         `json:"hour"`
	Frames  int64          `json:"frames"`
	Streams []*streamStats `json:"streams"`
}

func NewManifest(venue, hour string, expect []string) *Manifest {
	m := &Manifest{venue: venue, hour: hour, streams: map[string]*streamStats{}}
	for _, s := range expect {
		m.streams[s] = &streamStats{Stream: s}
	}
	return m
}

func (m *Manifest) Observe(now time.Time, frame []byte) {
	m.frames++

	var env struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(frame, &env); err != nil || env.Stream == "" {
		m.record(unknownStream, now.UnixMilli(), "receive", false, 0)
		return
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &data); err != nil {
		m.record(env.Stream, now.UnixMilli(), "receive", false, 0)
		return
	}

	eventMs, source := now.UnixMilli(), "receive"
	if v, ok := intField(data, "E"); ok {
		eventMs, source = v, "venue"
	}

	var id int64
	hasID := false
	if key := idField(env.Stream); key != "" {
		id, hasID = intField(data, key)
	}
	m.record(env.Stream, eventMs, source, hasID, id)
}

func (m *Manifest) record(stream string, eventMs int64, source string, hasID bool, id int64) {
	s, ok := m.streams[stream]
	if !ok {
		s = &streamStats{Stream: stream}
		m.streams[stream] = s
	}
	if s.Count == 0 {
		s.FirstEvent, s.EventSource, s.HasID, s.FirstID = eventMs, source, hasID, id
	}
	s.Count++
	s.LastEvent = eventMs
	if hasID {
		s.LastID = id
	}
}

func (m *Manifest) WriteFile(path string) error {
	doc := manifestDoc{Venue: m.venue, Hour: m.hour, Frames: m.frames}
	for _, s := range m.streams {
		doc.Streams = append(doc.Streams, s)
	}
	sort.Slice(doc.Streams, func(i, j int) bool { return doc.Streams[i].Stream < doc.Streams[j].Stream })

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func idField(stream string) string {
	switch {
	case strings.Contains(stream, "@markPrice"):
		return ""
	case strings.Contains(stream, "@aggTrade"):
		return "a"
	case strings.Contains(stream, "@trade"):
		return "t"
	case strings.Contains(stream, "@depth"):
		return "u"
	case strings.Contains(stream, "@bookTicker"):
		return "u"
	}
	return ""
}

func intField(data map[string]json.RawMessage, key string) (int64, bool) {
	raw, ok := data[key]
	if !ok {
		return 0, false
	}
	var v int64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	return v, true
}
