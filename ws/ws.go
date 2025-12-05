package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/coder/websocket"
	"github.com/ethereum/go-ethereum/common"
)

// Subscription represents an event subscription where events are
// delivered on a data channel.
type Subscription interface {
	// Unsubscribe cancels the sending of events to the data channel
	// and closes the error channel.
	Unsubscribe()

	// Err returns the subscription error channel. The error channel receives
	// a value if there is an issue with the subscription (e.g. the network
	// connection
	// delivering the events has been closed). Only one value will ever be sent.
	// The error channel is closed by Unsubscribe.
	Err() <-chan error
}

// subscription implements the Subscription interface
type subscription struct {
	cancel  func()
	errChan chan error
}

func (s *subscription) Unsubscribe() {
	s.cancel()
}

func (s *subscription) Err() <-chan error {
	return s.errChan
}

// ClientInterface defines the contract for WebSocket subscriptions
type ClientInterface interface {
	Start(ctx context.Context) error
	Close()
	SubscribeAllMids(
		ctx context.Context,
		ch chan<- AllMidsMessage,
	) (Subscription, error)
	SubscribeL2Book(
		ctx context.Context,
		coin string,
		ch chan<- L2BookMessage,
	) (Subscription, error)
	SubscribeTrades(
		ctx context.Context,
		coin string,
		ch chan<- TradesMessage,
	) (Subscription, error)
	SubscribeCandle(
		ctx context.Context,
		coin string,
		interval string,
		ch chan<- CandleMessage,
	) (Subscription, error)
	SubscribeBbo(
		ctx context.Context,
		coin string,
		ch chan<- BboMessage,
	) (Subscription, error)
	SubscribeActiveAssetCtx(
		ctx context.Context,
		coin string,
		ch chan<- ActiveAssetCtxMessage,
	) (Subscription, error)
	SubscribeUserEvents(
		ctx context.Context,
		user common.Address,
		ch chan<- UserEventsMessage,
	) (Subscription, error)
	SubscribeUserFills(
		ctx context.Context,
		user string,
		ch chan<- UserFillsMessage,
	) (Subscription, error)
	SubscribeOrderUpdates(
		ctx context.Context,
		user string,
		ch chan<- OrderUpdatesMessage,
	) (Subscription, error)
}

// Client manages WebSocket subscriptions and message routing
type Client struct {
	baseURL               string
	conn                  *websocket.Conn
	wsReady               bool
	subscriptionIDCounter int64
	activeSubscriptions   map[string][]*channelSubscription
	stopChan              chan struct{}
	wg                    sync.WaitGroup
	mu                    sync.RWMutex
}

// channelSubscription holds the internal channel for a subscription
type channelSubscription struct {
	internalChan any
	id           int64
}

// New creates a new WebSocket Client
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = constants.MAINNET_API_URL
	}

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

// Close closes the WebSocket connection and cleans up
func (m *Client) Close() {
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
		m.mu.RLock()
		conn := m.conn
		m.mu.RUnlock()

		if conn == nil {
			return
		}

		_, data, err := conn.Read(context.Background())
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

			ctx, cancel := context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
			err := conn.Write(ctx, websocket.MessageText, data)
			cancel()

			if err != nil {
				log.Printf("websocket ping error: %v", err)
				return
			}
		}
	}
}
