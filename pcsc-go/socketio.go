package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SocketIOClient is a minimal Socket.IO v4 client over WebSocket.
type SocketIOClient struct {
	rawURL    string
	namespace string
	conn      *websocket.Conn
	handlers  map[string]func(json.RawMessage)
	writeMu   sync.Mutex
	handlerMu sync.RWMutex
}

func NewSocketIOClient(rawURL, namespace string) *SocketIOClient {
	return &SocketIOClient{
		rawURL:    rawURL,
		namespace: namespace,
		handlers:  make(map[string]func(json.RawMessage)),
	}
}

func (c *SocketIOClient) On(event string, handler func(json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.handlers[event] = handler
}

func (c *SocketIOClient) Connect() error {
	return c.connect()
}

func (c *SocketIOClient) connect() error {
	u, err := url.Parse(c.rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/socket.io/?EIO=4&transport=websocket", scheme, u.Host)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	// Read Engine.IO OPEN packet (type 0)
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("read open: %w", err)
	}
	if len(msg) == 0 || msg[0] != '0' {
		conn.Close()
		return fmt.Errorf("unexpected open packet: %s", msg)
	}

	// Send Socket.IO CONNECT to namespace
	nsPart := ""
	if c.namespace != "" && c.namespace != "/" {
		nsPart = c.namespace + ","
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("40"+nsPart)); err != nil {
		conn.Close()
		return fmt.Errorf("send connect: %w", err)
	}

	// Wait for Socket.IO CONNECT ACK (40/namespace,{...})
	for {
		_, msg, err = conn.ReadMessage()
		if err != nil {
			conn.Close()
			return fmt.Errorf("read connect ack: %w", err)
		}
		s := string(msg)
		if strings.HasPrefix(s, "40") {
			break // connected
		}
		if strings.HasPrefix(s, "44") {
			conn.Close()
			return fmt.Errorf("namespace connect error: %s", s)
		}
		// Handle Engine.IO ping during handshake
		if s == "2" {
			conn.WriteMessage(websocket.TextMessage, []byte("3"))
		}
	}

	c.conn = conn
	c.fireHandler("connect", nil)
	go c.readLoop()
	return nil
}

func (c *SocketIOClient) readLoop() {
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			c.fireHandler("close", nil)
			go c.reconnectLoop()
			return
		}
		c.handlePacket(string(msg))
	}
}

func (c *SocketIOClient) handlePacket(msg string) {
	if len(msg) == 0 {
		return
	}
	switch msg[0] {
	case '2': // Engine.IO PING → respond with PONG
		c.writeMu.Lock()
		c.conn.WriteMessage(websocket.TextMessage, []byte("3"))
		c.writeMu.Unlock()
	case '3': // Engine.IO PONG (ignore)
	case '4': // Engine.IO MESSAGE → Socket.IO packet
		if len(msg) > 1 {
			c.handleSocketIO(msg[1:])
		}
	}
}

func (c *SocketIOClient) handleSocketIO(msg string) {
	if len(msg) == 0 {
		return
	}
	switch msg[0] {
	case '0': // CONNECT (re-ack, ignore)
	case '1': // DISCONNECT
		c.fireHandler("close", nil)
	case '2': // EVENT
		c.handleEvent(msg[1:])
	case '4': // CONNECT_ERROR
		log.Printf("Socket.IO connect error: %s", msg[1:])
	}
}

func (c *SocketIOClient) handleEvent(msg string) {
	// Strip namespace prefix: /server,["event",data]
	ns := c.namespace
	if ns != "" && ns != "/" && strings.HasPrefix(msg, ns+",") {
		msg = msg[len(ns)+1:]
	}

	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(msg), &raw); err != nil || len(raw) == 0 {
		return
	}

	var event string
	if err := json.Unmarshal(raw[0], &event); err != nil {
		return
	}

	var data json.RawMessage
	if len(raw) > 1 {
		data = raw[1]
	}

	c.fireHandler(event, data)
}

func (c *SocketIOClient) Emit(event string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	eventJSON, _ := json.Marshal(event)

	nsPart := ""
	if c.namespace != "" && c.namespace != "/" {
		nsPart = c.namespace + ","
	}

	pkt := fmt.Sprintf("42%s[%s,%s]", nsPart, eventJSON, payload)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, []byte(pkt))
}

func (c *SocketIOClient) fireHandler(event string, data json.RawMessage) {
	c.handlerMu.RLock()
	h, ok := c.handlers[event]
	c.handlerMu.RUnlock()
	if ok {
		h(data)
	}
}

func (c *SocketIOClient) reconnectLoop() {
	for {
		time.Sleep(2 * time.Second)
		log.Println("Reconnecting...")
		if err := c.connect(); err == nil {
			return
		}
		log.Println("Reconnect failed, retrying...")
	}
}
