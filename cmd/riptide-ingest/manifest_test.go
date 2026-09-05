package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	tradeFrame = `{"stream":"btcusdt@trade","data":{"e":"trade","E":1756000000001,"s":"BTCUSDT","t":100,"p":"80000.00","q":"0.1","T":1756000000000,"m":true,"M":true}}`
	depthFrame = `{"stream":"btcusdt@depth@100ms","data":{"e":"depthUpdate","E":1756000000010,"s":"BTCUSDT","U":900,"u":910,"b":[],"a":[]}}`
	aggFrame   = `{"stream":"btcusdt@aggTrade","data":{"e":"aggTrade","E":1756000000020,"s":"BTCUSDT","a":7000,"p":"80000.0","q":"1","f":1,"l":5,"T":1756000000019,"m":false}}`
	bookFrame  = `{"stream":"btcusdt@bookTicker","data":{"u":500,"s":"BTCUSDT","b":"79999.9","B":"2","a":"80000.1","A":"4"}}`
	markFrame  = `{"stream":"btcusdt@markPrice@1s","data":{"e":"markPriceUpdate","E":1756000000030,"s":"BTCUSDT","p":"80001.0","r":"0.0001","T":1756003600000}}`
)

func recv() time.Time { return time.Date(2026, 8, 31, 14, 5, 0, 0, time.UTC) }

func statsFor(t *testing.T, m *Manifest, stream string) *streamStats {
	t.Helper()
	s, ok := m.streams[stream]
	if !ok {
		t.Fatalf("no stats for %q; have %v", stream, m.streams)
	}
	return s
}

func TestManifestRecordsTradeStream(t *testing.T) {
	m := NewManifest("spot", "2026-08-31T14", nil)
	m.Observe(recv(), []byte(tradeFrame))

	s := statsFor(t, m, "btcusdt@trade")
	if s.Count != 1 {
		t.Errorf("count = %d, want 1", s.Count)
	}
	if s.FirstID != 100 || s.LastID != 100 || !s.HasID {
		t.Errorf("ids = %d..%d hasID=%v, want 100..100 true", s.FirstID, s.LastID, s.HasID)
	}
	if s.FirstEvent != 1756000000001 || s.LastEvent != 1756000000001 {
		t.Errorf("event ms = %d..%d, want 1756000000001", s.FirstEvent, s.LastEvent)
	}
	if s.EventSource != "venue" {
		t.Errorf("event source = %q, want venue", s.EventSource)
	}
}

func TestManifestDoesNotConfuseEventTypeWithEventTime(t *testing.T) {
	m := NewManifest("spot", "2026-08-31T14", nil)
	m.Observe(recv(), []byte(tradeFrame))

	s := statsFor(t, m, "btcusdt@trade")
	if s.FirstEvent != 1756000000001 {
		t.Fatalf(`event time = %d; the "e" event-type string leaked into "E"`, s.FirstEvent)
	}
	if s.FirstID != 100 {
		t.Errorf(`id = %d; the "T" trade-time leaked into "t"`, s.FirstID)
	}
}

func TestManifestKeepsFirstAdvancesLast(t *testing.T) {
	m := NewManifest("spot", "2026-08-31T14", nil)
	for i, id := range []int{100, 101, 102} {
		frame := fmt.Sprintf(`{"stream":"btcusdt@trade","data":{"e":"trade","E":%d,"t":%d}}`,
			1756000000001+i, id)
		m.Observe(recv(), []byte(frame))
	}

	s := statsFor(t, m, "btcusdt@trade")
	if s.Count != 3 {
		t.Errorf("count = %d, want 3", s.Count)
	}
	if s.FirstID != 100 || s.LastID != 102 {
		t.Errorf("ids = %d..%d, want 100..102", s.FirstID, s.LastID)
	}
}

func TestManifestTracksStreamsIndependently(t *testing.T) {
	m := NewManifest("perp", "2026-08-31T14", nil)
	m.Observe(recv(), []byte(depthFrame))
	m.Observe(recv(), []byte(aggFrame))
	m.Observe(recv(), []byte(aggFrame))

	if got := statsFor(t, m, "btcusdt@depth@100ms"); got.Count != 1 || got.LastID != 910 {
		t.Errorf("depth: count=%d lastID=%d, want 1 and 910", got.Count, got.LastID)
	}
	if got := statsFor(t, m, "btcusdt@aggTrade"); got.Count != 2 || got.LastID != 7000 {
		t.Errorf("aggTrade: count=%d lastID=%d, want 2 and 7000", got.Count, got.LastID)
	}
}

func TestManifestFallsBackToReceiveTimeForBookTicker(t *testing.T) {
	m := NewManifest("spot", "2026-08-31T14", nil)
	m.Observe(recv(), []byte(bookFrame))

	s := statsFor(t, m, "btcusdt@bookTicker")
	if s.EventSource != "receive" {
		t.Errorf("event source = %q, want receive", s.EventSource)
	}
	if s.FirstEvent != recv().UnixMilli() {
		t.Errorf("event ms = %d, want receive clock %d", s.FirstEvent, recv().UnixMilli())
	}
	if s.FirstID != 500 || !s.HasID {
		t.Errorf("bookTicker should still carry its u id, got %d hasID=%v", s.FirstID, s.HasID)
	}
}

func TestManifestRecordsNoIDForMarkPrice(t *testing.T) {
	m := NewManifest("perp", "2026-08-31T14", nil)
	m.Observe(recv(), []byte(markFrame))

	s := statsFor(t, m, "btcusdt@markPrice@1s")
	if s.HasID {
		t.Errorf("markPrice has no sequence id, got %d", s.FirstID)
	}
	if s.EventSource != "venue" || s.FirstEvent != 1756000000030 {
		t.Errorf("event = %d from %q, want 1756000000030 from venue", s.FirstEvent, s.EventSource)
	}
}

func TestManifestBucketsUnparseableFramesSoCountsReconcile(t *testing.T) {
	m := NewManifest("spot", "2026-08-31T14", nil)
	m.Observe(recv(), []byte(tradeFrame))
	m.Observe(recv(), []byte(`{"result":null,"id":1}`))
	m.Observe(recv(), []byte(`not json at all`))

	if m.frames != 3 {
		t.Errorf("frames = %d, want 3", m.frames)
	}
	var total int64
	for _, s := range m.streams {
		total += s.Count
	}
	if total != m.frames {
		t.Errorf("stream counts total %d but %d frames were observed", total, m.frames)
	}
	if _, ok := m.streams["?"]; !ok {
		t.Error(`unrecognised frames should land in the "?" bucket`)
	}
}

func TestManifestWriteFileIsSortedAndComplete(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest("spot", "2026-08-31T14", nil)
	m.Observe(recv(), []byte(markFrame))
	m.Observe(recv(), []byte(tradeFrame))
	m.Observe(recv(), []byte(bookFrame))

	path := filepath.Join(dir, "2026-08-31T14-spot.manifest.json")
	if err := m.WriteFile(path); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Venue   string `json:"venue"`
		Hour    string `json:"hour"`
		Frames  int64  `json:"frames"`
		Streams []struct {
			Stream string `json:"stream"`
			Count  int64  `json:"count"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Venue != "spot" || doc.Hour != "2026-08-31T14" || doc.Frames != 3 {
		t.Errorf("doc header = %+v", doc)
	}
	want := []string{"btcusdt@bookTicker", "btcusdt@markPrice@1s", "btcusdt@trade"}
	if len(doc.Streams) != len(want) {
		t.Fatalf("got %d streams, want %d", len(doc.Streams), len(want))
	}
	for i, w := range want {
		if doc.Streams[i].Stream != w {
			t.Errorf("stream %d = %q, want %q (sorted)", i, doc.Streams[i].Stream, w)
		}
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file was left behind")
	}
}

func TestManifestReportsExpectedStreamsThatNeverArrive(t *testing.T) {
	m := NewManifest("perp", "2026-08-31T14", []string{
		"btcusdt@aggTrade", "btcusdt@bookTicker", "btcusdt@markPrice@1s",
	})
	m.Observe(recv(), []byte(bookFrame))

	silent := statsFor(t, m, "btcusdt@aggTrade")
	if silent.Count != 0 {
		t.Errorf("count = %d, want 0", silent.Count)
	}
	if _, ok := m.streams["btcusdt@markPrice@1s"]; !ok {
		t.Error("a subscribed stream that sent nothing must still appear in the manifest")
	}
	if got := statsFor(t, m, "btcusdt@bookTicker"); got.Count != 1 {
		t.Errorf("arriving stream count = %d, want 1", got.Count)
	}
}

func TestManifestPopulatesFirstFieldsForExpectedStream(t *testing.T) {
	m := NewManifest("spot", "2026-08-31T14", []string{"btcusdt@trade"})
	m.Observe(recv(), []byte(tradeFrame))

	s := statsFor(t, m, "btcusdt@trade")
	if s.FirstID != 100 || s.FirstEvent != 1756000000001 || s.EventSource != "venue" {
		t.Errorf("pre-seeded stream lost its first observation: %+v", s)
	}
}
