package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg, err := Load()
	if err != nil {
		log.Fatal("config: ", err)
	}

	venues := []Venue{Spot, Perp}
	urls := make([]string, len(venues))
	names := make([][]string, len(venues))
	for i, v := range venues {
		if urls[i], err = v.StreamURL(cfg.Symbols); err != nil {
			log.Fatal("streams: ", err)
		}
		if names[i], err = v.StreamNames(cfg.Symbols); err != nil {
			log.Fatal("streams: ", err)
		}
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	for i, v := range venues {
		log.Printf("%s: %d streams (%d symbols x %d)", v.Name,
			len(cfg.Symbols)*len(v.Suffixes), len(cfg.Symbols), len(v.Suffixes))
		_ = urls[i]
	}
	log.Printf("writing to %s", cfg.OutputDir)

	var wg sync.WaitGroup
	for i, v := range venues {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := collect(ctx, cfg.OutputDir, v, urls[i], names[i]); err != nil {
				log.Printf("%s: stopping: %v", v.Name, err)
				cancel()
			}
		}()
	}
	wg.Wait()
	log.Println("stopped")
}

func collect(ctx context.Context, dir string, v Venue, url string, expect []string) error {
	tape := NewTape(dir, v.Name, expect)
	defer func() {
		if err := tape.Close(); err != nil {
			log.Printf("%s: closing tape: %v", v.Name, err)
		}
	}()

	c := NewCollector(url, tape.Write)
	c.OnConnect = func() { log.Printf("%s: connected", v.Name) }
	c.OnDropped = func(err error) { log.Printf("%s: disconnected: %v", v.Name, err) }
	return c.Run(ctx)
}
