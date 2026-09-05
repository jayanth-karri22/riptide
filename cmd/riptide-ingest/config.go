package main

import (
	"os"
	"strings"
)

type Config struct {
	OutputDir string
	Symbols   []string
}

func opt(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func optList(key string, def []string) []string {
	var out []string
	for _, s := range strings.Split(os.Getenv(key), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func Load() (*Config, error) {
	return &Config{
		OutputDir: opt("RIPTIDE_OUTPUT_DIR", "data"),
		Symbols: optList("RIPTIDE_SYMBOLS", []string{
			"btcusdt", "ethusdt", "solusdt", "xrpusdt", "bnbusdt",
			"dogeusdt", "adausdt", "avaxusdt", "linkusdt", "ltcusdt",
		}),
	}, nil
}
