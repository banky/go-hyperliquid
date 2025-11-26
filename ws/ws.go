package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Manager manages WebSocket subscriptions and message routing
type Manager struct {
	baseURL               string
	conn                  *websocket.Conn
	wsReady               bool
	subscriptionIDCounter int
	queuedSubscriptions   []queuedSubscription
	activeSubscriptions   map[string][]subscription
	stopChan              chan struct{}
	wg                    sync.WaitGroup
	mu                    sync.RWMutex
}

type subscription struct {
	callback any
	id       int
}

type queuedSubscription struct {
	sub      Subscription
	callback any
	id       int
}

// New creates a new WebSocket manager
func New(baseURL string) *Manager {
	return &Manager{
		baseURL:             baseURL,
		activeSubscriptions: make(map[string][]subscription),
		stopChan:            make(chan struct{}),
	}
}

// Start initializes the WebSocket connection and starts the read/ping loops
func (m *Manager) Start(ctx context.Context) error {
	u, err := url.Parse(m.baseURL)
	if err != nil {
		return fmt.Errorf("parse base URL %q: %w", m.baseURL, err)
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		// if you want to be strict instead:
		// return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
		u.Scheme = "ws"
	}

	// make sure we append "/ws" correctly, without double slashes
	u.Path = path.Join(u.Path, "ws")

	wsURL := u.String()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to websocket: %w", err)
	}

	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()

	m.wg.Add(2)
	go m.readLoop()
	go m.pingLoop()

	return nil
}

// Stop closes the WebSocket connection and cleans up
func (m *Manager) Stop() {
	close(m.stopChan)

	m.mu.Lock()
	if m.conn != nil {
		m.conn.Close(websocket.StatusNormalClosure, "closing")
	}
	m.mu.Unlock()

	m.wg.Wait()
}

// readLoop handles incoming messages from the WebSocket
func (m *Manager) readLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopChan:
			return
		default:
		}

		m.mu.RLock()
		conn := m.conn
		m.mu.RUnlock()

		if conn == nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, data, err := conn.Read(ctx)
		cancel()

		if err != nil {
			// Normal closure or context cancellation - exit gracefully
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			log.Printf("websocket read error: %v", err)
			return
		}

		message := string(data)
		if message == "Websocket connection established." {
			log.Println("websocket connection established")
			m.mu.Lock()
			m.wsReady = true
			queued := m.queuedSubscriptions
			m.queuedSubscriptions = nil
			m.mu.Unlock()

			// Process queued subscriptions
			for _, qs := range queued {
				m.subscribe(qs.sub, qs.callback, qs.id)
			}
			continue
		}

		m.handleMessage(data)
	}
}

// pingLoop sends periodic pings to keep the connection alive
func (m *Manager) pingLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(50 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.mu.RLock()
			conn := m.conn
			m.mu.RUnlock()

			if conn == nil {
				return
			}

			msg := map[string]string{"method": "ping"}
			data, _ := json.Marshal(msg)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := conn.Write(ctx, websocket.MessageText, data)
			cancel()

			if err != nil {
				log.Printf("websocket ping error: %v", err)
				return
			}
		}
	}
}

// handleMessage processes an incoming WebSocket message and routes it to callbacks
func (m *Manager) handleMessage(data []byte) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("failed to unmarshal ws message: %v", err)
		return
	}

	channel, ok := raw["channel"].(string)
	if !ok {
		log.Println("websocket message missing channel field")
		return
	}

	// Handle pong messages
	if channel == "pong" {
		log.Println("websocket received pong")
		return
	}

	// Route message to appropriate handler based on channel type
	switch channel {
	case "allMids":
		m.handleAllMids(raw)
	case "l2Book":
		m.handleL2Book(raw)
	case "trades":
		m.handleTrades(raw)
	case "user":
		m.handleUserEvents(raw)
	case "userFills":
		m.handleUserFills(raw)
	case "bbo":
		m.handleBbo(raw)
	case "candle":
		m.handleCandle(raw)
	case "orderUpdates":
		m.handleOrderUpdates(raw, "orderUpdates")
	case "userFundings":
		m.handleUserFundings(raw)
	case "userNonFundingLedgerUpdates":
		m.handleUserNonFundingLedgerUpdates(raw)
	case "webData2":
		m.handleWebData2(raw)
	case "activeAssetCtx", "activeSpotAssetCtx":
		m.handleActiveAssetCtx(raw, channel)
	case "activeAssetData":
		m.handleActiveAssetData(raw)
	default:
		log.Printf("websocket unknown channel: %s", channel)
	}
}

// Helper functions to handle each message type and route to callbacks

func (m *Manager) handleAllMids(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)
	var msg AllMidsMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("failed to unmarshal allMids message: %v", err)
		return
	}

	m.routeMessage("allMids", msg)
}

func (m *Manager) handleL2Book(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)
	var msg L2BookMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("failed to unmarshal l2Book message: %v", err)
		return
	}

	identifier := fmt.Sprintf("l2Book:%s", strings.ToLower(msg.Coin))
	m.routeMessage(identifier, msg)
}

func (m *Manager) handleTrades(raw map[string]any) {
	dataRaw, ok := raw["data"].([]any)
	if !ok || len(dataRaw) == 0 {
		return
	}

	// Extract trades and get coin from first trade
	var trades []Trade
	dataBytes, _ := json.Marshal(dataRaw)
	if err := json.Unmarshal(dataBytes, &trades); err != nil {
		log.Printf("failed to unmarshal trades message: %v", err)
		return
	}

	if len(trades) == 0 {
		return
	}

	msg := TradesMessage{Trades: trades}
	identifier := fmt.Sprintf("trades:%s", strings.ToLower(trades[0].Coin))
	m.routeMessage(identifier, msg)
}

func (m *Manager) handleUserEvents(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)
	var msg UserEventsMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("failed to unmarshal userEvents message: %v", err)
		return
	}

	m.routeMessage("userEvents", msg)
}

func (m *Manager) handleUserFills(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)
	var msg UserFillsMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("failed to unmarshal userFills message: %v", err)
		return
	}

	identifier := fmt.Sprintf("userFills:%s", strings.ToLower(msg.User))
	m.routeMessage(identifier, msg)
}

func (m *Manager) handleBbo(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)
	var msg BboMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("failed to unmarshal bbo message: %v", err)
		return
	}

	identifier := fmt.Sprintf("bbo:%s", strings.ToLower(msg.Coin))
	m.routeMessage(identifier, msg)
}

func (m *Manager) handleCandle(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)
	var msg CandleMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("failed to unmarshal candle message: %v", err)
		return
	}

	identifier := fmt.Sprintf("candle:%s,%s", strings.ToLower(msg.S), msg.I)
	m.routeMessage(identifier, msg)
}

func (m *Manager) handleOrderUpdates(raw map[string]any, identifier string) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msg := OrderUpdatesMessage(dataRaw.(map[string]any))
	m.routeMessage(identifier, msg)
}

func (m *Manager) handleUserFundings(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	dataMap := dataRaw.(map[string]any)
	msg := UserFundingsMessage(dataMap)

	// Try to extract user for identifier
	if user, ok := dataMap["user"].(string); ok {
		identifier := fmt.Sprintf("userFundings:%s", strings.ToLower(user))
		m.routeMessage(identifier, msg)
	}
}

func (m *Manager) handleUserNonFundingLedgerUpdates(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	dataMap := dataRaw.(map[string]any)
	msg := UserNonFundingLedgerUpdatesMessage(dataMap)

	// Try to extract user for identifier
	if user, ok := dataMap["user"].(string); ok {
		identifier := fmt.Sprintf("userNonFundingLedgerUpdates:%s", strings.ToLower(user))
		m.routeMessage(identifier, msg)
	}
}

func (m *Manager) handleWebData2(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	dataMap := dataRaw.(map[string]any)
	msg := WebData2Message(dataMap)

	// Try to extract user for identifier
	if user, ok := dataMap["user"].(string); ok {
		identifier := fmt.Sprintf("webData2:%s", strings.ToLower(user))
		m.routeMessage(identifier, msg)
	}
}

func (m *Manager) handleActiveAssetCtx(raw map[string]any, channel string) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)

	if channel == "activeSpotAssetCtx" {
		var msg ActiveSpotAssetCtxMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Printf("failed to unmarshal activeSpotAssetCtx message: %v", err)
			return
		}
		identifier := fmt.Sprintf("activeAssetCtx:%s", strings.ToLower(msg.Coin))
		m.routeMessage(identifier, msg)
	} else {
		var msg ActiveAssetCtxMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Printf("failed to unmarshal activeAssetCtx message: %v", err)
			return
		}
		identifier := fmt.Sprintf("activeAssetCtx:%s", strings.ToLower(msg.Coin))
		m.routeMessage(identifier, msg)
	}
}

func (m *Manager) handleActiveAssetData(raw map[string]any) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msgBytes, _ := json.Marshal(dataRaw)
	var msg ActiveAssetDataMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("failed to unmarshal activeAssetData message: %v", err)
		return
	}

	identifier := fmt.Sprintf("activeAssetData:%s,%s", strings.ToLower(msg.Coin), strings.ToLower(msg.User))
	m.routeMessage(identifier, msg)
}

// routeMessage routes a message to all callbacks registered for that identifier
func (m *Manager) routeMessage(identifier string, msg any) {
	m.mu.RLock()
	callbacks := m.activeSubscriptions[identifier]
	m.mu.RUnlock()

	if len(callbacks) == 0 {
		log.Printf("websocket message from unexpected subscription: %s", identifier)
		return
	}

	for _, sub := range callbacks {
		m.callCallback(sub.callback, msg)
	}
}

// callCallback executes a callback with the appropriate type assertion
func (m *Manager) callCallback(callback any, msg any) {
	switch cb := callback.(type) {
	case func(*AllMidsMessage):
		if msg, ok := msg.(AllMidsMessage); ok {
			go cb(&msg)
		}
	case func(*L2BookMessage):
		if msg, ok := msg.(L2BookMessage); ok {
			go cb(&msg)
		}
	case func(*TradesMessage):
		if msg, ok := msg.(TradesMessage); ok {
			go cb(&msg)
		}
	case func(*UserEventsMessage):
		if msg, ok := msg.(UserEventsMessage); ok {
			go cb(&msg)
		}
	case func(*UserFillsMessage):
		if msg, ok := msg.(UserFillsMessage); ok {
			go cb(&msg)
		}
	case func(*BboMessage):
		if msg, ok := msg.(BboMessage); ok {
			go cb(&msg)
		}
	case func(*CandleMessage):
		if msg, ok := msg.(CandleMessage); ok {
			go cb(&msg)
		}
	case func(OrderUpdatesMessage):
		if msg, ok := msg.(OrderUpdatesMessage); ok {
			go cb(msg)
		}
	case func(UserFundingsMessage):
		if msg, ok := msg.(UserFundingsMessage); ok {
			go cb(msg)
		}
	case func(UserNonFundingLedgerUpdatesMessage):
		if msg, ok := msg.(UserNonFundingLedgerUpdatesMessage); ok {
			go cb(msg)
		}
	case func(WebData2Message):
		if msg, ok := msg.(WebData2Message); ok {
			go cb(msg)
		}
	case func(*ActiveAssetCtxMessage):
		if msg, ok := msg.(ActiveAssetCtxMessage); ok {
			go cb(&msg)
		}
	case func(*ActiveSpotAssetCtxMessage):
		if msg, ok := msg.(ActiveSpotAssetCtxMessage); ok {
			go cb(&msg)
		}
	case func(*ActiveAssetDataMessage):
		if msg, ok := msg.(ActiveAssetDataMessage); ok {
			go cb(&msg)
		}
	default:
		log.Printf("unknown callback type: %T", callback)
	}
}

// ===== Type-safe subscription methods =====

// SubscribeAllMids subscribes to all mid-prices
func (m *Manager) SubscribeAllMids(callback func(*AllMidsMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(AllMidsSubscription{}, callback, id)
	return id
}

// SubscribeL2Book subscribes to level 2 order book for a coin
func (m *Manager) SubscribeL2Book(coin string, callback func(*L2BookMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(L2BookSubscription{Coin: coin}, callback, id)
	return id
}

// SubscribeTrades subscribes to trades for a coin
func (m *Manager) SubscribeTrades(coin string, callback func(*TradesMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(TradesSubscription{Coin: coin}, callback, id)
	return id
}

// SubscribeUserEvents subscribes to user events
func (m *Manager) SubscribeUserEvents(user string, callback func(*UserEventsMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(UserEventsSubscription{User: user}, callback, id)
	return id
}

// SubscribeUserFills subscribes to user fills
func (m *Manager) SubscribeUserFills(user string, callback func(*UserFillsMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(UserFillsSubscription{User: user}, callback, id)
	return id
}

// SubscribeCandle subscribes to candle data
func (m *Manager) SubscribeCandle(coin string, interval string, callback func(*CandleMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(CandleSubscription{Coin: coin, Interval: interval}, callback, id)
	return id
}

// SubscribeOrderUpdates subscribes to order updates
func (m *Manager) SubscribeOrderUpdates(user string, callback func(OrderUpdatesMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(OrderUpdatesSubscription{User: user}, callback, id)
	return id
}

// SubscribeUserFundings subscribes to user fundings
func (m *Manager) SubscribeUserFundings(user string, callback func(UserFundingsMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(UserFundingsSubscription{User: user}, callback, id)
	return id
}

// SubscribeUserNonFundingLedgerUpdates subscribes to non-funding ledger updates
func (m *Manager) SubscribeUserNonFundingLedgerUpdates(user string, callback func(UserNonFundingLedgerUpdatesMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(UserNonFundingLedgerUpdatesSubscription{User: user}, callback, id)
	return id
}

// SubscribeWebData2 subscribes to web data
func (m *Manager) SubscribeWebData2(user string, callback func(WebData2Message)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(WebData2Subscription{User: user}, callback, id)
	return id
}

// SubscribeBbo subscribes to best bid/offer data
func (m *Manager) SubscribeBbo(coin string, callback func(*BboMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(BboSubscription{Coin: coin}, callback, id)
	return id
}

// SubscribeActiveAssetCtx subscribes to active asset context
func (m *Manager) SubscribeActiveAssetCtx(coin string, callback func(*ActiveAssetCtxMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(ActiveAssetCtxSubscription{Coin: coin}, callback, id)
	return id
}

// SubscribeActiveAssetData subscribes to active asset data
func (m *Manager) SubscribeActiveAssetData(coin string, user string, callback func(*ActiveAssetDataMessage)) int {
	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	m.subscribe(ActiveAssetDataSubscription{Coin: coin, User: user}, callback, id)
	return id
}

// subscribe handles the internal subscription logic
func (m *Manager) subscribe(sub Subscription, callback any, id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	identifier := sub.identifier()

	if !m.wsReady {
		log.Println("enqueueing subscription")
		m.queuedSubscriptions = append(m.queuedSubscriptions, queuedSubscription{
			sub:      sub,
			callback: callback,
			id:       id,
		})
		return
	}

	log.Println("subscribing")

	// Check for duplicate restrictions
	if identifier == "userEvents" || identifier == "orderUpdates" {
		if len(m.activeSubscriptions[identifier]) != 0 {
			log.Printf("cannot subscribe to %s multiple times", identifier)
			return
		}
	}

	m.activeSubscriptions[identifier] = append(m.activeSubscriptions[identifier], subscription{
		callback: callback,
		id:       id,
	})

	// Send subscription message to server
	msg := map[string]any{"method": "subscribe", "subscription": sub.subscriptionPayload()}
	data, _ := json.Marshal(msg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.conn.Write(ctx, websocket.MessageText, data)
}

// Unsubscribe removes a subscription
func (m *Manager) Unsubscribe(sub Subscription, subscriptionID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.wsReady {
		log.Println("cannot unsubscribe before websocket connected")
		return false
	}

	identifier := sub.identifier()
	activeSubscriptions := m.activeSubscriptions[identifier]

	// Filter out the subscription
	newActiveSubscriptions := make([]subscription, 0)
	for _, s := range activeSubscriptions {
		if s.id != subscriptionID {
			newActiveSubscriptions = append(newActiveSubscriptions, s)
		}
	}

	// If no more subscriptions for this identifier, send unsubscribe
	if len(newActiveSubscriptions) == 0 {
		msg := map[string]any{"method": "unsubscribe", "subscription": sub.subscriptionPayload()}
		data, _ := json.Marshal(msg)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.conn.Write(ctx, websocket.MessageText, data)
	}

	m.activeSubscriptions[identifier] = newActiveSubscriptions
	return len(activeSubscriptions) != len(newActiveSubscriptions)
}
