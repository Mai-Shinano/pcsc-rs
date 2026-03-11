package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// gitDescribe is set via: go build -ldflags "-X main.gitDescribe=$(git describe --tags --long --dirty)"
var gitDescribe = "dev"

func main() {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Fatal(".env file found but could not be loaded")
		}
	}

	if os.Getenv("PASS") == "" {
		fmt.Println("The environment variable Password (PASS) is not specified.")
		os.Exit(95)
	}

	start()
}

func start() {
	if err := update(); err != nil {
		log.Printf("Update check failed: %v", err)
	}

	var (
		mu     sync.RWMutex
		status = getStatus()
	)

	// Monitor goroutine: refresh system status every second
	go func() {
		for {
			time.Sleep(time.Second)
			s := getStatus()
			mu.Lock()
			status = s
			mu.Unlock()
		}
	}()

	pcscURI := os.Getenv("PCSC_URI")
	if pcscURI == "" {
		pcscURI = "https://pcss.eov2.com"
	}

	fmt.Println("This OS is supported!")
	fmt.Printf("Hello, world! %s\n", pcscURI)

	client := NewSocketIOClient(pcscURI, "/server")

	client.On("connect", func(_ json.RawMessage) {
		fmt.Println("Connected")
	})

	client.On("close", func(_ json.RawMessage) {
		fmt.Println("Disconnected")
	})

	client.On("hi", func(data json.RawMessage) {
		fmt.Printf("Received: %s\n", data)
		fmt.Print("hi from server")
		s := getStatus()
		if err := client.Emit("hi", StatusDataWithPass{
			SystemStatus: s,
			Pass:         os.Getenv("PASS"),
		}); err != nil {
			log.Printf("Failed to emit hi: %v", err)
		}
	})

	client.On("sync", func(data json.RawMessage) {
		fmt.Printf("Received: %s\n", data)
		mu.RLock()
		s := status
		mu.RUnlock()
		if err := client.Emit("sync", s); err != nil {
			log.Printf("Failed to emit sync: %v", err)
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}

	select {}
}
