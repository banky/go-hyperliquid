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

// Subscription represents an event subscription where events are
// delivered on a data channel.
type Subscription interface {
	// Unsubscribe cancels the sending of events to the data channel
	// and closes the error channel.
	Unsubscribe()

	// Err returns the subscription error channel. The error channel receives
	// a value if there is an issue with the subscription (e.g. the network connection
	// delivering the events has been closed). Only one value will ever be sent.
	// The error channel is closed by Unsubscribe.
	Err() <-chan error
}

// subscription implements the Subscription interface
type subscription struct {
	unsubscribe func()
	errChan     chan error
	closeOnce   sync.Once
}

func (s *subscription) Unsubscribe() {
	s.closeOnce.Do(func() {
		s.unsubscribe()
		close(s.errChan)
	})
}

func (s *subscription) Err() <-chan error {
	return s.errChan
}

// ClientInterface defines the contract for WebSocket subscriptions
type ClientInterface interface {
	Start(ctx context.Context) error
	Stop()
	SubscribeAllMids(ctx context.Context, ch chan<- AllMidsMessage) (Subscription, error)
	SubscribeL2Book(ctx context.Context, coin string, ch chan<- L2BookMessage) (Subscription, error)
	SubscribeTrades(ctx context.Context, coin string, ch chan<- TradesMessage) (Subscription, error)
	SubscribeCandle(ctx context.Context, coin string, interval string, ch chan<- CandleMessage) (Subscription, error)
	SubscribeBbo(ctx context.Context, coin string, ch chan<- BboMessage) (Subscription, error)
	SubscribeActiveAssetCtx(ctx context.Context, coin string, ch chan<- ActiveAssetCtxMessage) (Subscription, error)
	SubscribeUserEvents(ctx context.Context, user string, ch chan<- UserEventsMessage) (Subscription, error)
	SubscribeUserFills(ctx context.Context, user string, ch chan<- UserFillsMessage) (Subscription, error)
	SubscribeOrderUpdates(ctx context.Context, user string, ch chan<- OrderUpdatesMessage) (Subscription, error)
}

// Client manages WebSocket subscriptions and message routing
type Client struct {
	baseURL               string
	conn                  *websocket.Conn
	wsReady               bool
	subscriptionIDCounter int
	activeSubscriptions   map[string][]*channelSubscription
	stopChan              chan struct{}
	wg                    sync.WaitGroup
	mu                    sync.RWMutex
}

// channelSubscription holds the internal buffered channel for a subscription
type channelSubscription struct {
	// Typed channel for messages (buffered, capacity 10)
	internalChan interface{}
	id           int
}

// New creates a new WebSocket Client
func New(baseURL string) *Client {
	return &Client{
		baseURL:             baseURL,
		activeSubscriptions: make(map[string][]*channelSubscription),
		stopChan:            make(chan struct{}),
	}
}

// Start initializes the WebSocket connection and starts the read/ping loops
func (m *Client) Start(ctx context.Context) error {
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
func (m *Client) Stop() {
	close(m.stopChan)

	m.mu.Lock()
	if m.conn != nil {
		m.conn.Close(websocket.StatusNormalClosure, "closing")
	}
	m.mu.Unlock()

	m.wg.Wait()
}

// readLoop handles incoming messages from the WebSocket
func (m *Client) readLoop() {
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
			m.mu.Unlock()
			continue
		}

		m.handleMessage(data)
	}
}

// pingLoop sends periodic pings to keep the connection alive
func (m *Client) pingLoop() {
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
func (m *Client) handleMessage(data []byte) {
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

func (m *Client) handleAllMids(raw map[string]any) {
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

func (m *Client) handleL2Book(raw map[string]any) {
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

func (m *Client) handleTrades(raw map[string]any) {
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

func (m *Client) handleUserEvents(raw map[string]any) {
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

func (m *Client) handleUserFills(raw map[string]any) {
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

func (m *Client) handleBbo(raw map[string]any) {
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

func (m *Client) handleCandle(raw map[string]any) {
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

func (m *Client) handleOrderUpdates(raw map[string]any, identifier string) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msg := OrderUpdatesMessage(dataRaw.(map[string]any))
	m.routeMessage(identifier, msg)
}

func (m *Client) handleUserFundings(raw map[string]any) {
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

func (m *Client) handleUserNonFundingLedgerUpdates(raw map[string]any) {
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

func (m *Client) handleWebData2(raw map[string]any) {
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

func (m *Client) handleActiveAssetCtx(raw map[string]any, channel string) {
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

func (m *Client) handleActiveAssetData(raw map[string]any) {
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

// routeMessage routes a message to all subscriptions registered for that identifier
func (m *Client) routeMessage(identifier string, msg any) {
	m.mu.RLock()
	subscriptions := m.activeSubscriptions[identifier]
	m.mu.RUnlock()

	if len(subscriptions) == 0 {
		log.Printf("websocket message from unexpected subscription: %s", identifier)
		return
	}

	for _, sub := range subscriptions {
		// Non-blocking send to internal channel
		select {
		case sub.internalChan.(chan any) <- msg:
			// Message sent
		default:
			// Channel full, drop message
			log.Printf("subscription internal channel full for %s (id: %d)", identifier, sub.id)
		}
	}
}

// ===== Type-safe subscription methods =====

// SubscribeAllMids subscribes to all mid-prices
func (m *Client) SubscribeAllMids(ctx context.Context, ch chan<- AllMidsMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	if err := m.subscribe(AllMidsSubscription{}, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(AllMidsSubscription{}, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeL2Book subscribes to level 2 order book for a coin
func (m *Client) SubscribeL2Book(ctx context.Context, coin string, ch chan<- L2BookMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := L2BookSubscription{Coin: coin}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeTrades subscribes to trades for a coin
func (m *Client) SubscribeTrades(ctx context.Context, coin string, ch chan<- TradesMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := TradesSubscription{Coin: coin}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeUserEvents subscribes to user events
func (m *Client) SubscribeUserEvents(ctx context.Context, user string, ch chan<- UserEventsMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := UserEventsSubscription{User: user}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeUserFills subscribes to user fills
func (m *Client) SubscribeUserFills(ctx context.Context, user string, ch chan<- UserFillsMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := UserFillsSubscription{User: user}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeCandle subscribes to candle data
func (m *Client) SubscribeCandle(ctx context.Context, coin string, interval string, ch chan<- CandleMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := CandleSubscription{Coin: coin, Interval: interval}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeOrderUpdates subscribes to order updates
func (m *Client) SubscribeOrderUpdates(ctx context.Context, user string, ch chan<- OrderUpdatesMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := OrderUpdatesSubscription{User: user}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeUserFundings subscribes to user fundings
func (m *Client) SubscribeUserFundings(ctx context.Context, user string, ch chan<- UserFundingsMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := UserFundingsSubscription{User: user}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeUserNonFundingLedgerUpdates subscribes to non-funding ledger updates
func (m *Client) SubscribeUserNonFundingLedgerUpdates(ctx context.Context, user string, ch chan<- UserNonFundingLedgerUpdatesMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := UserNonFundingLedgerUpdatesSubscription{User: user}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeWebData2 subscribes to web data
func (m *Client) SubscribeWebData2(ctx context.Context, user string, ch chan<- WebData2Message) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := WebData2Subscription{User: user}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeBbo subscribes to best bid/offer data
func (m *Client) SubscribeBbo(ctx context.Context, coin string, ch chan<- BboMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := BboSubscription{Coin: coin}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeActiveAssetCtx subscribes to active asset context
func (m *Client) SubscribeActiveAssetCtx(ctx context.Context, coin string, ch chan<- ActiveAssetCtxMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := ActiveAssetCtxSubscription{Coin: coin}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// SubscribeActiveAssetData subscribes to active asset data
func (m *Client) SubscribeActiveAssetData(ctx context.Context, coin string, user string, ch chan<- ActiveAssetDataMessage) (Subscription, error) {
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.subscriptionIDCounter++
	id := m.subscriptionIDCounter
	m.mu.Unlock()

	sub := ActiveAssetDataSubscription{Coin: coin, User: user}
	if err := m.subscribe(sub, ch, id); err != nil {
		close(errChan)
		return nil, err
	}

	subscriptionImpl := &subscription{
		errChan: errChan,
		unsubscribe: func() {
			m.unsubscribeInternal(sub, id)
		},
	}

	go func() {
		<-ctx.Done()
		// Try to send error, but recover if channel is already closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, that's fine
				}
			}()
			select {
			case errChan <- ctx.Err():
			default:
			}
		}()
		subscriptionImpl.Unsubscribe()
	}()

	return subscriptionImpl, nil
}

// subscribe handles the internal subscription logic with channel-based delivery
func (m *Client) subscribe(sub SubscriptionType, subscriberChan interface{}, id int) error {
	identifier := sub.identifier()

	// Create internal buffered channel for this subscription
	// We use reflection to create a properly typed buffered channel
	internalChan := make(chan any, 10)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate restrictions
	if identifier == "userEvents" || identifier == "orderUpdates" {
		if len(m.activeSubscriptions[identifier]) != 0 {
			return fmt.Errorf("cannot subscribe to %s multiple times", identifier)
		}
	}

	// Add to active subscriptions
	m.activeSubscriptions[identifier] = append(m.activeSubscriptions[identifier], &channelSubscription{
		internalChan: internalChan,
		id:           id,
	})

	// Launch delivery goroutine that forwards from internal channel to subscriber channel
	go m.deliveryLoop(internalChan, subscriberChan)

	// Send subscription message to server (if connected)
	if m.conn != nil {
		msg := map[string]any{"method": "subscribe", "subscription": sub.subscriptionPayload()}
		data, _ := json.Marshal(msg)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.conn.Write(ctx, websocket.MessageText, data)
	}

	return nil
}

// deliveryLoop forwards messages from the internal channel to the subscriber's channel
func (m *Client) deliveryLoop(internalChan chan any, subscriberChan interface{}) {
	// We need to use a reflect-based approach to send to the subscriber channel
	// since we don't know the exact type at compile time
	for msg := range internalChan {
		// Try to send to the subscriber channel without blocking
		// This is a best-effort approach; if the channel is closed, the send will panic
		// which is caught by the recover mechanism in case of closed channel
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was closed, exit gracefully
					return
				}
			}()

			// Use type switch to handle different channel types
			switch ch := subscriberChan.(type) {
			case chan AllMidsMessage:
				if m, ok := msg.(AllMidsMessage); ok {
					ch <- m
				}
			case chan L2BookMessage:
				if m, ok := msg.(L2BookMessage); ok {
					ch <- m
				}
			case chan TradesMessage:
				if m, ok := msg.(TradesMessage); ok {
					ch <- m
				}
			case chan UserEventsMessage:
				if m, ok := msg.(UserEventsMessage); ok {
					ch <- m
				}
			case chan UserFillsMessage:
				if m, ok := msg.(UserFillsMessage); ok {
					ch <- m
				}
			case chan BboMessage:
				if m, ok := msg.(BboMessage); ok {
					ch <- m
				}
			case chan CandleMessage:
				if m, ok := msg.(CandleMessage); ok {
					ch <- m
				}
			case chan OrderUpdatesMessage:
				if m, ok := msg.(OrderUpdatesMessage); ok {
					ch <- m
				}
			case chan UserFundingsMessage:
				if m, ok := msg.(UserFundingsMessage); ok {
					ch <- m
				}
			case chan UserNonFundingLedgerUpdatesMessage:
				if m, ok := msg.(UserNonFundingLedgerUpdatesMessage); ok {
					ch <- m
				}
			case chan WebData2Message:
				if m, ok := msg.(WebData2Message); ok {
					ch <- m
				}
			case chan ActiveAssetCtxMessage:
				if m, ok := msg.(ActiveAssetCtxMessage); ok {
					ch <- m
				}
			case chan ActiveSpotAssetCtxMessage:
				if m, ok := msg.(ActiveSpotAssetCtxMessage); ok {
					ch <- m
				}
			case chan ActiveAssetDataMessage:
				if m, ok := msg.(ActiveAssetDataMessage); ok {
					ch <- m
				}
			}
		}()
	}
}

// unsubscribeInternal removes a subscription and closes its internal channel
func (m *Client) unsubscribeInternal(sub SubscriptionType, subscriptionID int) bool {
	m.mu.Lock()
	identifier := sub.identifier()
	activeSubscriptions := m.activeSubscriptions[identifier]

	// Find and close the internal channel
	var internalChan chan any
	newActiveSubscriptions := make([]*channelSubscription, 0)
	for _, s := range activeSubscriptions {
		if s.id == subscriptionID {
			internalChan = s.internalChan.(chan any)
		} else {
			newActiveSubscriptions = append(newActiveSubscriptions, s)
		}
	}

	if internalChan != nil {
		close(internalChan)
	}

	// If no more subscriptions for this identifier, send unsubscribe (if connected)
	if len(newActiveSubscriptions) == 0 && m.conn != nil {
		msg := map[string]any{"method": "unsubscribe", "subscription": sub.subscriptionPayload()}
		data, _ := json.Marshal(msg)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		m.mu.Unlock()
		defer cancel()
		m.conn.Write(ctx, websocket.MessageText, data)
		m.mu.Lock()
	}

	m.activeSubscriptions[identifier] = newActiveSubscriptions
	m.mu.Unlock()

	return internalChan != nil
}
