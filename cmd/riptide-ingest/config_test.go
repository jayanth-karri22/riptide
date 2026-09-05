package main

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("RIPTIDE_OUTPUT_DIR", "")
	t.Setenv("RIPTIDE_SYMBOLS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OutputDir != "data" {
		t.Errorf("OutputDir = %q, want data", cfg.OutputDir)
	}
	if len(cfg.Symbols) != 10 || cfg.Symbols[0] != "btcusdt" {
		t.Errorf("Symbols = %v", cfg.Symbols)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("RIPTIDE_OUTPUT_DIR", "/tmp/riptide")
	t.Setenv("RIPTIDE_SYMBOLS", " BTCUSDT , ethusdt ,, ")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OutputDir != "/tmp/riptide" {
		t.Errorf("OutputDir = %q", cfg.OutputDir)
	}
	if len(cfg.Symbols) != 2 || cfg.Symbols[0] != "BTCUSDT" || cfg.Symbols[1] != "ethusdt" {
		t.Errorf("Symbols = %v, want the two non-blank entries trimmed", cfg.Symbols)
	}
}
