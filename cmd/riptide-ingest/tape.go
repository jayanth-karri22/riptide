package main

import (
	"bufio"
	"os"
	"path/filepath"
	"time"
)

const tapeHourLayout = "2006-01-02T15"

type Tape struct {
	dir    string
	venue  string
	hour   string
	file   *os.File
	buf    *bufio.Writer
	man    *Manifest
	expect []string
}

func NewTape(dir, venue string, expect []string) *Tape {
	return &Tape{dir: dir, venue: venue, expect: expect}
}

func (t *Tape) Write(now time.Time, frame []byte) error {
	hour := now.UTC().Format(tapeHourLayout)
	if t.file == nil || t.hour != hour {
		if err := t.finalise(); err != nil {
			return err
		}
		if err := t.open(hour); err != nil {
			return err
		}
	}
	if _, err := t.buf.Write(frame); err != nil {
		return err
	}
	if err := t.buf.WriteByte('\n'); err != nil {
		return err
	}
	t.man.Observe(now, frame)
	return nil
}

func (t *Tape) Close() error {
	return t.finalise()
}

func (t *Tape) path(hour string) string {
	return filepath.Join(t.dir, hour+"-"+t.venue+".jsonl")
}

func (t *Tape) manifestPath(hour string) string {
	return filepath.Join(t.dir, hour+"-"+t.venue+".manifest.json")
}

func (t *Tape) open(hour string) error {
	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(t.path(hour)+".part", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	t.file, t.buf, t.hour = f, bufio.NewWriterSize(f, 1<<20), hour
	t.man = NewManifest(t.venue, hour, t.expect)
	return nil
}

func (t *Tape) finalise() error {
	if t.file == nil {
		return nil
	}
	part, final := t.path(t.hour)+".part", t.path(t.hour)
	man, manPath := t.man, t.manifestPath(t.hour)

	err := t.buf.Flush()
	if cerr := t.file.Close(); err == nil {
		err = cerr
	}
	t.file, t.buf, t.man, t.hour = nil, nil, nil, ""

	if err != nil {
		return err
	}
	if err := os.Rename(part, final); err != nil {
		return err
	}
	return man.WriteFile(manPath)
}
