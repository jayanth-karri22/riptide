package main

import (
	"errors"
	"fmt"
	"strings"
)

// Venue is one Binance perp and spot with different host, name and limits with similar behaviour
type Venue struct {
	Name       string
	Base       string
	Suffixes   []string
	MaxStreams int
}

var (
	Spot = Venue{
		Name:       "spot",
		Base:       "wss://stream.binance.com:9443/stream",
		MaxStreams: 1024,
		Suffixes:   []string{"@trade", "@bookTicker", "@depth@100ms"},
	}

	Perp = Venue{
		Name:       "perp",
		Base:       "wss://fstream.binance.com/public/stream",
		MaxStreams: 200,
		Suffixes:   []string{"@aggTrade", "@bookTicker", "@depth@100ms", "@markPrice@1s"},
	}
)

func (v Venue) StreamURL(symbols []string) (string, error) {
	names, err := v.StreamNames(symbols)
	if err != nil {
		return "", err
	}
	return v.Base + "?streams=" + strings.Join(names, "/"), nil
}

func (v Venue) StreamNames(symbols []string) ([]string, error) {
	clean := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		clean = append(clean, s)
	}
	if len(clean) == 0 {
		return nil, errors.New("no symbols configured")
	}
	if n := len(clean) * len(v.Suffixes); n > v.MaxStreams {
		return nil, fmt.Errorf("%s: %d streams exceeds the %d per-connection cap", v.Name, n, v.MaxStreams)
	}

	names := make([]string, 0, len(clean)*len(v.Suffixes))
	for _, s := range clean {
		for _, suffix := range v.Suffixes {
			names = append(names, s+suffix)
		}
	}
	return names, nil
}
