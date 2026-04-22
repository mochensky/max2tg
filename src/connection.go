package src

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultReconnectDelay     = 5 * time.Second
	maxReconnectDelay         = 60 * time.Second
	reconnectDelayMultiplier  = 2.0
	pingInterval              = 30 * time.Second
	websocketHandshakeTimeout = 10 * time.Second
	websocketWriteTimeout     = 10 * time.Second
)

type ClientHandler interface {
	GetFromWebSocketHandlers() []func(string)
	GetToWebSocketHandlers() []func(string)
	GetDisconnectedHandlers() []func(string)
	GetBeforeReconnectHandlers() []func(string)
	GetAfterReconnectHandlers() []func()
	SetMe(Me)
	SetChats([]Chat)
	SetConnectionTime(time.Time)
}

type Connection struct {
	config              *Config
	client              ClientHandler
	conn                *websocket.Conn
	connected           bool
	requestBuilder      *RequestBuilder
	messageQueue        chan WebSocketPayload
	responseFutures     map[int]chan WebSocketPayload
	pingTicker          *time.Ticker
	lastPongTime        time.Time
	reconnectAttempts   int
	currentDelay        time.Duration
	maxBackoffDelay     time.Duration
	manualStop          bool
	performingReconnect bool
	reconnectLock       sync.Mutex
	stopCh              chan struct{}
	stopOnce            sync.Once

	mu sync.RWMutex
}

func NewConnection(config *Config, client ClientHandler) *Connection {
	return &Connection{
		config:          config,
		client:          client,
		connected:       false,
		requestBuilder:  NewRequestBuilder(config),
		messageQueue:    make(chan WebSocketPayload, 1000),
		responseFutures: make(map[int]chan WebSocketPayload),
		currentDelay:    defaultReconnectDelay,
		maxBackoffDelay: maxReconnectDelay,
	}
}

func (c *Connection) closeStopCh() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

func (c *Connection) Connect() error {
	if c.connected {
		return nil
	}

	if c.config.UserAgent.UserAgent == "" {
		return ErrInvalidUserAgent
	}

	headers := map[string][]string{
		"User-Agent":            {c.config.UserAgent.UserAgent},
		"Accept":                {"*/*"},
		"Accept-Language":       {c.config.UserAgent.DeviceLocale + ";q=0.5"},
		"Accept-Encoding":       {"gzip, deflate, br, zstd"},
		"Sec-WebSocket-Version": {"13"},
		"Origin":                {"https://web.max.ru"},
		"Sec-Fetch-Dest":        {"empty"},
		"Sec-Fetch-Mode":        {"websocket"},
		"Sec-Fetch-Site":        {"cross-site"},
		"Pragma":                {"no-cache"},
		"Cache-Control":         {"no-cache"},
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: websocketHandshakeTimeout,
	}

	conn, _, err := dialer.Dial(WEBSOCKET_URL, headers)
	if err != nil {
		return NewConnectionError("failed to connect: " + err.Error())
	}

	c.conn = conn
	c.connected = true
	c.reconnectAttempts = 0
	c.currentDelay = defaultReconnectDelay

	c.requestBuilder = NewRequestBuilder(c.config)

	c.stopCh = make(chan struct{})
	c.stopOnce = sync.Once{}

	go c.receiveMessages(c.stopCh)

	if err := c.sendInit(); err != nil {
		c.closeStopCh()
		conn.Close()
		c.connected = false
		return err
	}

	c.startPing(c.stopCh)

	Logf("Connected to WebSocket")
	return nil
}

func (c *Connection) sendInit() error {
	initData := c.requestBuilder.Init()
	if err := c.send(initData); err != nil {
		return err
	}

	response, err := c.receiveWithTimeout(10 * time.Second)
	if err != nil {
		return err
	}

	opcodeFloat, _ := response["opcode"].(float64)
	cmdFloat, _ := response["cmd"].(float64)
	opcode := int(opcodeFloat)
	cmd := int(cmdFloat)
	if opcode != int(INIT) || cmd != 1 {
		return NewInvalidResponseError("invalid initialization response")
	}

	return nil
}

func (c *Connection) receiveWithTimeout(timeout time.Duration) (WebSocketPayload, error) {
	select {
	case payload, ok := <-c.messageQueue:
		if !ok {
			return nil, NewConnectionError("message queue closed")
		}
		return payload, nil
	case <-c.stopCh:
		return nil, NewConnectionError("receiver stopped")
	case <-time.After(timeout):
		return nil, NewTimeoutError("receive timeout")
	}
}

func (c *Connection) Authenticate(chatsCount int) (WebSocketPayload, error) {
	if !c.connected {
		return nil, NewConnectionError("WebSocket not connected")
	}

	authData := c.requestBuilder.Authenticate(chatsCount)
	response, err := c.sendAndReceive(authData, 10*time.Second)
	if err != nil {
		return nil, err
	}

	opcodeFloat, _ := response["opcode"].(float64)
	cmdFloat, _ := response["cmd"].(float64)
	opcode := int(opcodeFloat)
	cmd := int(cmdFloat)
	if opcode != int(AUTH) || cmd != 1 {
		return nil, NewInvalidResponseError("authentication failed: invalid response")
	}

	payload, ok := response["payload"].(map[string]interface{})
	if !ok {
		return nil, NewInvalidTokenError("authentication failed: invalid response payload")
	}

	if _, hasProfile := payload["profile"]; !hasProfile {
		return nil, NewInvalidTokenError("authentication failed: no profile data")
	}

	if token, ok := payload["token"].(string); ok && token != "" {
		c.config.Token = token
		Logf("Token updated after authentication")
	}

	return response, nil
}

func (c *Connection) startPing(stopCh <-chan struct{}) {
	c.mu.Lock()
	c.lastPongTime = time.Now()
	c.mu.Unlock()

	ticker := time.NewTicker(pingInterval)
	c.pingTicker = ticker

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				if !c.connected {
					return
				}

				c.mu.RLock()
				lastPong := c.lastPongTime
				c.mu.RUnlock()

				if time.Since(lastPong) > c.config.PingTimeout {
					Logf("Ping timeout: server did not respond for %v, initiating reconnect", time.Since(lastPong).Round(time.Second))
					c.connected = false
					if c.conn != nil {
						c.conn.Close()
					}
					go c.HandleReconnect("Ping timeout: server stopped responding")
					return
				}

				pingData := c.requestBuilder.Ping()
				if err := c.send(pingData); err != nil {
					Logf("Ping failed: %v", err)
					return
				}
			}
		}
	}()
}

func (c *Connection) receiveMessages(stopCh <-chan struct{}) {
	defer c.closeStopCh()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if c.connected {
				Logf("WebSocket read error: %v", err)
				c.connected = false
				go c.HandleReconnect("WebSocket connection closed")
			}
			return
		}

		Logf("↓ %s", string(message))

		var payload WebSocketPayload
		if err := json.Unmarshal(message, &payload); err != nil {
			Logf("Failed to parse message: %v", err)
			continue
		}

		if c.client != nil {
			for _, handler := range c.client.GetFromWebSocketHandlers() {
				handler(string(message))
			}
		}

		c.mu.Lock()
		c.lastPongTime = time.Now()
		c.mu.Unlock()

		if seq, ok := parseInt(payload["seq"]); ok && c.isResponse(payload) {
			c.mu.Lock()
			if future, exists := c.responseFutures[seq]; exists {
				select {
				case future <- payload:
				default:
				}
				delete(c.responseFutures, seq)
			}
			c.mu.Unlock()
		}

		select {
		case c.messageQueue <- payload:
		case <-stopCh:
			return
		default:
			Logf("Message queue full, dropping message")
		}
	}
}

func (c *Connection) isResponse(payload WebSocketPayload) bool {
	cmd, ok := parseInt(payload["cmd"])
	return ok && cmd == 1
}

func (c *Connection) send(payload WebSocketPayload) error {
	if !c.connected {
		return NewConnectionError("WebSocket not connected")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	Logf("↑ %s", string(data))

	if c.client != nil {
		for _, handler := range c.client.GetToWebSocketHandlers() {
			handler(string(data))
		}
	}

	c.conn.SetWriteDeadline(time.Now().Add(websocketWriteTimeout))
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return NewConnectionError("failed to send: " + err.Error())
	}

	return nil
}

func (c *Connection) sendAndReceive(payload WebSocketPayload, timeout time.Duration) (WebSocketPayload, error) {
	if !c.connected {
		return nil, NewConnectionError("WebSocket not connected")
	}

	seq := payload["seq"].(int)
	responseChan := make(chan WebSocketPayload, 1)

	c.mu.Lock()
	c.responseFutures[seq] = responseChan
	c.mu.Unlock()

	if err := c.send(payload); err != nil {
		c.mu.Lock()
		delete(c.responseFutures, seq)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case response, ok := <-responseChan:
		if !ok {
			return nil, NewConnectionError("connection lost while waiting for response")
		}
		return response, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.responseFutures, seq)
		c.mu.Unlock()
		return nil, NewTimeoutError("request timeout")
	}
}

func (c *Connection) Receive() (WebSocketPayload, error) {
	select {
	case payload, ok := <-c.messageQueue:
		if !ok {
			return nil, NewConnectionError("message queue closed")
		}
		return payload, nil
	case <-c.stopCh:
		return nil, NewConnectionError("receiver stopped")
	}
}

func (c *Connection) HandleReconnect(reason string) {
	c.reconnectLock.Lock()

	if c.manualStop {
		c.manualStop = false
		c.connected = false
		c.reconnectLock.Unlock()
		return
	}

	if c.performingReconnect {
		Logf("Reconnect already in progress, skipping duplicate call")
		c.reconnectLock.Unlock()
		return
	}

	c.performingReconnect = true
	c.reconnectLock.Unlock()

	c.mu.Lock()
	for seq, ch := range c.responseFutures {
		close(ch)
		delete(c.responseFutures, seq)
	}
	c.mu.Unlock()

	if c.client != nil {
		for _, handler := range c.client.GetDisconnectedHandlers() {
			handler(reason)
		}
		for _, handler := range c.client.GetBeforeReconnectHandlers() {
			handler(reason)
		}
	}

	c.reconnectAttempts++
	Logf("Reconnect attempt #%d due to: %s", c.reconnectAttempts, reason)

	delay := c.currentDelay
	if c.currentDelay < c.maxBackoffDelay {
		c.currentDelay = time.Duration(float64(c.currentDelay) * reconnectDelayMultiplier)
		if c.currentDelay > c.maxBackoffDelay {
			c.currentDelay = c.maxBackoffDelay
		}
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false

	for {
		if err := c.Connect(); err != nil {
			Logf("Reconnect failed: %v, retrying...", err)
			retryDelay := time.Duration(float64(c.currentDelay) * 1.5)
			time.Sleep(retryDelay)
			continue
		}

		if c.client != nil {
			response, err := c.Authenticate(40)
			if err != nil {
				Logf("Authentication after reconnect failed: %v", err)
				c.closeStopCh()
				if c.conn != nil {
					c.conn.Close()
				}
				c.connected = false
				time.Sleep(2 * time.Second)
				continue
			}

			if payload, ok := response["payload"].(map[string]interface{}); ok {
				c.client.SetMe(parseProfile(payload))
				if chatsData, ok := payload["chats"].([]interface{}); ok {
					chats := parseChats(castToMapArray(chatsData))
					c.client.SetChats(chats)
				}
			}
			c.client.SetConnectionTime(time.Now())

			for _, handler := range c.client.GetAfterReconnectHandlers() {
				handler()
			}
		}

		break
	}

	c.reconnectLock.Lock()
	c.performingReconnect = false
	c.reconnectLock.Unlock()
}

func castToMapArray(arr []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, len(arr))
	for i, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			result[i] = m
		}
	}
	return result
}

func (c *Connection) Close() {
	c.reconnectLock.Lock()
	c.manualStop = true
	c.reconnectLock.Unlock()

	if c.stopCh != nil {
		c.closeStopCh()
	}

	if c.conn != nil && c.connected {
		c.conn.Close()
	}
	c.connected = false
	c.requestBuilder = NewRequestBuilder(c.config)

	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}

	oldQueue := c.messageQueue
	c.messageQueue = make(chan WebSocketPayload, 1000)
	close(oldQueue)
	for range oldQueue {
	}

	c.mu.Lock()
	for seq, ch := range c.responseFutures {
		close(ch)
		delete(c.responseFutures, seq)
	}
	c.mu.Unlock()

	c.reconnectAttempts = 0
	c.currentDelay = defaultReconnectDelay
	c.performingReconnect = false
}

func (c *Connection) IsConnected() bool {
	return c.connected
}

func (c *Connection) IsReconnecting() bool {
	c.reconnectLock.Lock()
	defer c.reconnectLock.Unlock()
	return c.performingReconnect
}

func (c *Connection) GetRequestBuilder() *RequestBuilder {
	return c.requestBuilder
}
