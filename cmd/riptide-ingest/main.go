package main

import (
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func stream(url string, f *os.File) error {
	con, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	defer con.Close()
	log.Println("connected to", url)

	for {
		_, msg, err := con.ReadMessage()
		if err != nil {
			return err
		}
		if _, err := f.Write(append(msg, '\n')); err != nil {
			return err
		}
	}

}

func main() {
	f, err := os.OpenFile("trades.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Open File:", err)
	}
	defer f.Close()

	url := "wss://stream.binance.com:9443/ws/btcusdt@trade"
	for {
		err := stream(url, f)
		log.Println("disconnected", err, "-reconnecting in 5s")
		time.Sleep(5 * time.Second)
	}
}
