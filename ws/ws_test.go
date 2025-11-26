package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// ===== Subscription Identifier Tests =====

func TestSubscriptionIdentifiers(t *testing.T) {
	tests := []struct {
		name       string
		sub        Subscription
		expectedID string
	}{
		{
			name:       "AllMids",
			sub:        AllMidsSubscription{},
			expectedID: "allMids",
		},
		{
			name:       "L2Book",
			sub:        L2BookSubscription{Coin: "BTC"},
			expectedID: "l2Book:btc",
		},
		{
			name:       "Trades",
			sub:        TradesSubscription{Coin: "ETH"},
			expectedID: "trades:eth",
		},
		{
			name:       "UserEvents",
			sub:        UserEventsSubscription{User: "0xABC"},
			expectedID: "userEvents",
		},
		{
			name:       "UserFills",
			sub:        UserFillsSubscription{User: "0xABC"},
			expectedID: "userFills:0xabc",
		},
		{
			name:       "Candle",
			sub:        CandleSubscription{Coin: "BTC", Interval: "1h"},
			expectedID: "candle:btc,1h",
		},
		{
			name:       "Bbo",
			sub:        BboSubscription{Coin: "SOL"},
			expectedID: "bbo:sol",
		},
		{
			name:       "ActiveAssetCtx",
			sub:        ActiveAssetCtxSubscription{Coin: "BTC"},
			expectedID: "activeAssetCtx:btc",
		},
		{
			name:       "ActiveAssetData",
			sub:        ActiveAssetDataSubscription{Coin: "ETH", User: "0xXYZ"},
			expectedID: "activeAssetData:eth,0xxyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sub.identifier()
			if got != tt.expectedID {
				t.Errorf("identifier() = %q, want %q", got, tt.expectedID)
			}
		})
	}
}

// ===== Mock WebSocket Server =====

// mockWSServer simulates a Hyperliquid WebSocket server
type mockWSServer struct {
	server *httptest.Server
	url    string
}

func newMockWSServer(t *testing.T) *mockWSServer {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("websocket accept error: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "test complete")

		// Send connection established message
		conn.Write(context.Background(), websocket.MessageText, []byte("Websocket connection established."))

		// Handle subscription messages and send responses
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, data, err := conn.Read(ctx)
			cancel()

			if err != nil {
				return
			}

			var msg map[string]any
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			method, _ := msg["method"].(string)
			switch method {
			case "ping":
				pongMsg := map[string]string{"channel": "pong"}
				pongData, _ := json.Marshal(pongMsg)
				conn.Write(context.Background(), websocket.MessageText, pongData)
			case "subscribe":
				// Server acknowledges subscription
				_ = msg["subscription"]
			case "unsubscribe":
				// Server acknowledges unsubscription
				_ = msg["subscription"]
			}
		}
	}))

	return &mockWSServer{
		server: server,
		url:    "http" + strings.TrimPrefix(server.URL, "http"),
	}
}

func (s *mockWSServer) close() {
	s.server.Close()
}

// ===== Manager Lifecycle Tests =====

func TestManagerStartStop(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give it time to process the connection message
	time.Sleep(100 * time.Millisecond)

	manager.Stop()
}

// ===== Subscription Queue Tests =====

func TestQueuedSubscriptions(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)

	// Subscribe before connection is ready
	id := manager.SubscribeAllMids(func(msg *AllMidsMessage) {})

	if id != 1 {
		t.Errorf("expected subscription ID 1, got %d", id)
	}

	// Should be queued, not active yet
	manager.mu.RLock()
	if len(manager.activeSubscriptions["allMids"]) != 0 {
		t.Error("expected no active subscriptions before connection")
	}
	if len(manager.queuedSubscriptions) != 1 {
		t.Error("expected 1 queued subscription")
	}
	manager.mu.RUnlock()

	// Now connect
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give it time to process queued subscriptions
	time.Sleep(100 * time.Millisecond)

	// Check that subscription was processed
	manager.mu.RLock()
	if len(manager.activeSubscriptions["allMids"]) != 1 {
		t.Error("expected 1 active subscription after connection")
	}
	if len(manager.queuedSubscriptions) != 0 {
		t.Error("expected 0 queued subscriptions after connection")
	}
	manager.mu.RUnlock()

	manager.Stop()
}

// ===== Message Routing Tests =====

func TestL2BookMessageRouting(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	// Start manager with custom message handler
	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Subscribe to L2 book
	var received *L2BookMessage
	var mu sync.Mutex
	manager.SubscribeL2Book("BTC", func(msg *L2BookMessage) {
		mu.Lock()
		received = msg
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	// Manually inject a message for testing
	msgData := map[string]any{
		"channel": "l2Book",
		"data": map[string]any{
			"coin": "BTC",
			"levels": [][]map[string]any{
				{
					{
						"px": "50000",
						"sz": "1.5",
						"n":  5,
					},
				},
				{
					{
						"px": "50100",
						"sz": "2.0",
						"n":  3,
					},
				},
			},
			"time": 1234567890,
		},
	}
	msgBytes, _ := json.Marshal(msgData)
	manager.handleMessage(msgBytes)

	// Wait for async callback
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if received == nil {
		t.Fatal("callback not called")
	}
	if received.Coin != "BTC" {
		t.Errorf("expected coin BTC, got %s", received.Coin)
	}
	if received.Time != 1234567890 {
		t.Errorf("expected time 1234567890, got %d", received.Time)
	}
	mu.Unlock()

	manager.Stop()
}

func TestTradesMessageRouting(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	var received *TradesMessage
	var mu sync.Mutex
	manager.SubscribeTrades("ETH", func(msg *TradesMessage) {
		mu.Lock()
		received = msg
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	msgData := map[string]any{
		"channel": "trades",
		"data": []any{
			map[string]any{
				"coin": "ETH",
				"side": "A",
				"px":   "3000",
				"sz":   10,
				"hash": "0xabc123",
				"time": 1234567890,
			},
			map[string]any{
				"coin": "ETH",
				"side": "B",
				"px":   "3001",
				"sz":   5,
				"hash": "0xdef456",
				"time": 1234567891,
			},
		},
	}
	msgBytes, _ := json.Marshal(msgData)
	manager.handleMessage(msgBytes)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if received == nil {
		t.Fatal("callback not called")
	}
	if len(received.Trades) != 2 {
		t.Errorf("expected 2 trades, got %d", len(received.Trades))
	}
	if received.Trades[0].Coin != "ETH" {
		t.Errorf("expected coin ETH, got %s", received.Trades[0].Coin)
	}
	mu.Unlock()

	manager.Stop()
}

// ===== Multiplexing Constraint Tests =====

func TestUserEventsDuplicateSubscription(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// First subscription should work
	id1 := manager.SubscribeUserEvents("0xABC", func(msg *UserEventsMessage) {})
	if id1 == 0 {
		t.Error("expected valid subscription ID")
	}

	time.Sleep(100 * time.Millisecond)

	// Second subscription to same identifier should fail
	// (manager won't add it)
	id2 := manager.SubscribeUserEvents("0xDEF", func(msg *UserEventsMessage) {})
	if id2 == 0 {
		t.Error("expected valid subscription ID")
	}

	manager.mu.RLock()
	count := len(manager.activeSubscriptions["userEvents"])
	manager.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 active userEvents subscription, got %d", count)
	}

	manager.Stop()
}

// ===== Add/Remove Subscription Tests =====

func TestUnsubscribe(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Subscribe to multiple coins
	id1 := manager.SubscribeL2Book("BTC", func(msg *L2BookMessage) {})
	_ = manager.SubscribeL2Book("BTC", func(msg *L2BookMessage) {})
	_ = manager.SubscribeL2Book("ETH", func(msg *L2BookMessage) {})

	time.Sleep(100 * time.Millisecond)

	// Check initial state
	manager.mu.RLock()
	btcSubs := len(manager.activeSubscriptions["l2Book:btc"])
	manager.mu.RUnlock()

	if btcSubs != 2 {
		t.Errorf("expected 2 BTC subscriptions, got %d", btcSubs)
	}

	// Unsubscribe from one BTC subscription
	found := manager.Unsubscribe(L2BookSubscription{Coin: "BTC"}, id1)
	if !found {
		t.Error("expected unsubscribe to find subscription")
	}

	time.Sleep(50 * time.Millisecond)

	// Check that one was removed
	manager.mu.RLock()
	btcSubs = len(manager.activeSubscriptions["l2Book:btc"])
	manager.mu.RUnlock()

	if btcSubs != 1 {
		t.Errorf("expected 1 BTC subscription after unsubscribe, got %d", btcSubs)
	}

	// ETH should be unaffected
	manager.mu.RLock()
	ethSubs := len(manager.activeSubscriptions["l2Book:eth"])
	manager.mu.RUnlock()

	if ethSubs != 1 {
		t.Errorf("expected 1 ETH subscription, got %d", ethSubs)
	}

	manager.Stop()
}

// ===== Multiple Callbacks Per Subscription =====

func TestMultipleCallbacksPerSubscription(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	var called1, called2 bool
	var mu sync.Mutex

	// Multiple callbacks for same coin
	manager.SubscribeL2Book("BTC", func(msg *L2BookMessage) {
		mu.Lock()
		called1 = true
		mu.Unlock()
	})

	manager.SubscribeL2Book("BTC", func(msg *L2BookMessage) {
		mu.Lock()
		called2 = true
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	// Send message
	msgData := map[string]any{
		"channel": "l2Book",
		"data": map[string]any{
			"coin": "BTC",
			"levels": [][]map[string]any{
				{},
				{},
			},
			"time": 1234567890,
		},
	}
	msgBytes, _ := json.Marshal(msgData)
	manager.handleMessage(msgBytes)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !called1 || !called2 {
		t.Errorf("both callbacks should be called: called1=%v, called2=%v", called1, called2)
	}
	mu.Unlock()

	manager.Stop()
}

// ===== Edge Cases =====

func TestEmptyTradesMessage(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	var callCount int
	manager.SubscribeTrades("ETH", func(msg *TradesMessage) {
		callCount++
	})

	time.Sleep(100 * time.Millisecond)

	// Send empty trades list - should not call callback
	msgData := map[string]any{
		"channel": "trades",
		"data":    []any{},
	}
	msgBytes, _ := json.Marshal(msgData)
	manager.handleMessage(msgBytes)

	time.Sleep(50 * time.Millisecond)

	if callCount != 0 {
		t.Errorf("expected no callback for empty trades, got %d calls", callCount)
	}

	manager.Stop()
}

func TestMissingDataField(t *testing.T) {
	server := newMockWSServer(t)
	defer server.close()

	manager := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := manager.Start(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Subscribe but don't expect callback
	var callCount int
	manager.SubscribeL2Book("BTC", func(msg *L2BookMessage) {
		callCount++
	})

	time.Sleep(100 * time.Millisecond)

	// Message missing data field - should not crash
	msgData := map[string]any{
		"channel": "l2Book",
	}
	msgBytes, _ := json.Marshal(msgData)
	manager.handleMessage(msgBytes)

	time.Sleep(50 * time.Millisecond)

	if callCount != 0 {
		t.Errorf("expected no callback for malformed message, got %d calls", callCount)
	}

	manager.Stop()
}

func TestSubscriptionPayload(t *testing.T) {
	tests := []struct {
		name         string
		sub          Subscription
		expectedKeys []string
	}{
		{
			name:         "L2Book includes type and coin",
			sub:          L2BookSubscription{Coin: "BTC"},
			expectedKeys: []string{"type", "coin"},
		},
		{
			name:         "Candle includes type, coin, and interval",
			sub:          CandleSubscription{Coin: "ETH", Interval: "1h"},
			expectedKeys: []string{"type", "coin", "interval"},
		},
		{
			name:         "ActiveAssetData includes type, coin, and user",
			sub:          ActiveAssetDataSubscription{Coin: "SOL", User: "0xABC"},
			expectedKeys: []string{"type", "coin", "user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := tt.sub.subscriptionPayload()
			payloadMap, ok := payload.(map[string]any)
			if !ok {
				t.Fatal("subscription payload is not a map")
			}

			for _, key := range tt.expectedKeys {
				if _, exists := payloadMap[key]; !exists {
					t.Errorf("expected key %q in payload", key)
				}
			}
		})
	}
}
