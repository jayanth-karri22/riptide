package main

import "os"

type Config struct {
	OutputPath string
	StreamURL  string
}

func opt(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() (*Config, error) {
	return &Config{
		OutputPath: opt("RIPTIDE_OUTPUT_PATH", "trades.jsonl"),
		StreamURL:  opt("RIPTIDE_STREAM_URL", "wss://stream.binance.com:9443/ws/btcusdt@trade"),
	}, nil
}
