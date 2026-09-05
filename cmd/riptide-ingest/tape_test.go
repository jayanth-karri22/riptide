package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func at(hour, min int) time.Time {
	return time.Date(2026, 8, 31, hour, min, 0, 0, time.UTC)
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTapeMarksOpenFileAsPart(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "spot", nil)
	defer tape.Close()

	if err := tape.Write(at(14, 5), []byte(`{"stream":"btcusdt@trade"}`)); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	if len(got) != 1 || got[0] != "2026-08-31T14-spot.jsonl.part" {
		t.Errorf("got %v, want [2026-08-31T14-spot.jsonl.part]", got)
	}
}

func TestTapeFinalisesOnClose(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "spot", nil)
	frame := []byte(`{"stream":"btcusdt@trade","data":{"t":1}}`)

	if err := tape.Write(at(14, 5), frame); err != nil {
		t.Fatal(err)
	}
	if err := tape.Close(); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	want := []string{"2026-08-31T14-spot.jsonl", "2026-08-31T14-spot.manifest.json"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
	if body, want := read(t, dir, got[0]), string(frame)+"\n"; body != want {
		t.Errorf("got %q, want %q", body, want)
	}
}

func TestTapeAppendsWithinSameHour(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "perp", nil)

	for _, f := range []string{`{"n":1}`, `{"n":2}`, `{"n":3}`} {
		if err := tape.Write(at(14, 5), []byte(f)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tape.Close(); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	if len(got) != 2 {
		t.Fatalf("got %v, want the tape and its manifest", got)
	}
	if body, want := read(t, dir, got[0]), "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n"; body != want {
		t.Errorf("got %q, want %q", body, want)
	}
}

func TestTapeRotatesOnHourBoundary(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "spot", nil)
	defer tape.Close()

	if err := tape.Write(at(14, 59), []byte(`{"hour":14}`)); err != nil {
		t.Fatal(err)
	}
	if err := tape.Write(at(15, 0), []byte(`{"hour":15}`)); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	want := []string{
		"2026-08-31T14-spot.jsonl",
		"2026-08-31T14-spot.manifest.json",
		"2026-08-31T15-spot.jsonl.part",
	}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("got %v, want %v", got, want)
	}
	if body, w := read(t, dir, want[0]), "{\"hour\":14}\n"; body != w {
		t.Errorf("rotated file: got %q, want %q", body, w)
	}
}

func TestTapeWritesFramesVerbatim(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "spot", nil)
	frame := []byte(`{"s":"BTCUSDT","p":"80577.75000000","note":"quote \" and ünïcode ✓"}`)

	if err := tape.Write(at(14, 5), frame); err != nil {
		t.Fatal(err)
	}
	if err := tape.Close(); err != nil {
		t.Fatal(err)
	}

	if body, want := read(t, dir, names(t, dir)[0]), string(frame)+"\n"; body != want {
		t.Errorf("frame was altered\n got %q\nwant %q", body, want)
	}
}

func TestTapeNamesFilesInUTC(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "spot", nil)
	defer tape.Close()

	ist := time.FixedZone("IST", 5*3600+1800)
	if err := tape.Write(time.Date(2026, 8, 31, 19, 35, 0, 0, ist), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	if len(got) != 1 || got[0] != "2026-08-31T14-spot.jsonl.part" {
		t.Errorf("got %v, want the 14:00 UTC hour, not the 19:00 local one", got)
	}
}

func TestTapeWritesNoManifestWhileHourIsOpen(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "spot", nil)
	defer tape.Close()

	if err := tape.Write(at(14, 5), []byte(tradeFrame)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-08-31T14-spot.manifest.json")); !os.IsNotExist(err) {
		t.Error("manifest must not appear until the tape is finalised")
	}
}

func TestTapeWritesManifestBesideFinalisedTape(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "spot", nil)
	defer tape.Close()

	for _, id := range []int{100, 101} {
		frame := fmt.Sprintf(`{"stream":"btcusdt@trade","data":{"e":"trade","E":%d,"t":%d}}`,
			1756000000001+id, id)
		if err := tape.Write(at(14, 5), []byte(frame)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tape.Write(at(15, 0), []byte(tradeFrame)); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "2026-08-31T14-spot.manifest.json"))
	if err != nil {
		t.Fatalf("no manifest beside the finalised tape: %v", err)
	}
	var doc struct {
		Venue   string `json:"venue"`
		Hour    string `json:"hour"`
		Frames  int64  `json:"frames"`
		Streams []struct {
			Stream  string `json:"stream"`
			Count   int64  `json:"count"`
			FirstID int64  `json:"first_id"`
			LastID  int64  `json:"last_id"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Venue != "spot" || doc.Hour != "2026-08-31T14" || doc.Frames != 2 {
		t.Errorf("header = %+v, want spot/2026-08-31T14/2 frames", doc)
	}
	if len(doc.Streams) != 1 || doc.Streams[0].FirstID != 100 || doc.Streams[0].LastID != 101 {
		t.Errorf("streams = %+v, want one stream spanning ids 100..101", doc.Streams)
	}
}

func TestTapeManifestListsSilentSubscriptions(t *testing.T) {
	dir := t.TempDir()
	tape := NewTape(dir, "perp", []string{"btcusdt@aggTrade", "btcusdt@markPrice@1s"})

	if err := tape.Write(at(14, 5), []byte(tradeFrame)); err != nil {
		t.Fatal(err)
	}
	if err := tape.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "2026-08-31T14-perp.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Streams []struct {
			Stream string `json:"stream"`
			Count  int64  `json:"count"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, s := range doc.Streams {
		counts[s.Stream] = s.Count
	}
	for _, want := range []string{"btcusdt@aggTrade", "btcusdt@markPrice@1s"} {
		if c, ok := counts[want]; !ok || c != 0 {
			t.Errorf("%s: present=%v count=%d, want present with count 0", want, ok, c)
		}
	}
	if counts["btcusdt@trade"] != 1 {
		t.Errorf("arriving stream count = %d, want 1", counts["btcusdt@trade"])
	}
}
