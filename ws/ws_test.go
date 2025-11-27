package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// ===== Subscription Identifier Tests =====

func TestSubscriptionIdentifiers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sub        SubscriptionType
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
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Logf("websocket accept error: %v", err)
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "test complete")

			// Send connection established message
			conn.Write(
				context.Background(),
				websocket.MessageText,
				[]byte("Websocket connection established."),
			)

			// Handle subscription messages and send responses
			for {
				ctx, cancel := context.WithTimeout(
					context.Background(),
					2*time.Second,
				)
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
					conn.Write(
						context.Background(),
						websocket.MessageText,
						pongData,
					)
				case "subscribe":
					// Server acknowledges subscription
					_ = msg["subscription"]
				case "unsubscribe":
					// Server acknowledges unsubscription
					_ = msg["subscription"]
				}
			}
		}),
	)

	return &mockWSServer{
		server: server,
		url:    "http" + strings.TrimPrefix(server.URL, "http"),
	}
}

func (s *mockWSServer) close() {
	s.server.Close()
}

// ===== Client Lifecycle Tests =====

func TestClientStartStop(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give it time to process the connection message
	time.Sleep(100 * time.Millisecond)

	client.Close()
}

// ===== Channel-Based Subscription Tests =====

func TestChannelSubscription(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Subscribe to AllMids with a channel
	msgChan := make(chan AllMidsMessage)
	sub, err := client.SubscribeAllMids(ctx, msgChan)
	if err != nil {
		t.Fatalf("SubscribeAllMids() failed: %v", err)
	}
	if sub == nil {
		t.Fatal("expected non-nil subscription")
	}

	time.Sleep(100 * time.Millisecond)

	// Check that subscription is active
	client.mu.RLock()
	if len(client.activeSubscriptions["allMids"]) != 1 {
		t.Error("expected 1 active allMids subscription")
	}
	client.mu.RUnlock()

	// Unsubscribe
	sub.Unsubscribe()

	time.Sleep(50 * time.Millisecond)

	// Check that subscription is gone
	client.mu.RLock()
	if len(client.activeSubscriptions["allMids"]) != 0 {
		t.Error("expected 0 active allMids subscriptions after unsubscribe")
	}
	client.mu.RUnlock()

	client.Close()
}

// ===== Message Routing Tests =====

func TestL2BookMessageRouting(t *testing.T) {
	t.Parallel()
	// Focus on testing message routing logic with predictable setup
	// client := New("http://localhost:8000") // URL doesn't matter, won't connect

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client.Start(ctx)

	// Create subscriber channel
	msgChan := make(chan L2BookMessage)
	sub, err := client.SubscribeL2Book(ctx, "BTC", msgChan)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	time.Sleep(10 * time.Millisecond)

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
	client.handleMessage(msgBytes)

	// Receive message from channel
	select {
	case received := <-msgChan:
		if received.Coin != "BTC" {
			t.Errorf("expected coin BTC, got %s", received.Coin)
		}
		if received.Time != 1234567890 {
			t.Errorf("expected time 1234567890, got %d", received.Time)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestTradesMessageRouting(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client.Start(ctx)

	// Create subscriber channel
	msgChan := make(chan TradesMessage)
	sub, err := client.SubscribeTrades(ctx, "ETH", msgChan)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	time.Sleep(10 * time.Millisecond)

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
	client.handleMessage(msgBytes)

	select {
	case received := <-msgChan:
		if len(received.Trades) != 2 {
			t.Errorf("expected 2 trades, got %d", len(received.Trades))
		}
		if received.Trades[0].Coin != "ETH" {
			t.Errorf("expected coin ETH, got %s", received.Trades[0].Coin)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// ===== Multiplexing Constraint Tests =====

func TestUserEventsDuplicateSubscription(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	// First subscription should work
	msgChan1 := make(chan UserEventsMessage)
	sub1, err := client.SubscribeUserEvents(
		context.Background(),
		"0xABC",
		msgChan1,
	)
	if err != nil {
		t.Fatalf("first SubscribeUserEvents() failed: %v", err)
	}
	defer sub1.Unsubscribe()

	time.Sleep(100 * time.Millisecond)

	// Second subscription to same channel (userEvents) should fail
	// because userEvents only allows one subscription
	msgChan2 := make(chan UserEventsMessage)
	sub2, err := client.SubscribeUserEvents(
		context.Background(),
		"0xDEF",
		msgChan2,
	)
	if err == nil {
		// If it succeeded, that's an error - userEvents should only allow 1
		sub2.Unsubscribe()
		t.Error("expected second userEvents subscription to fail")
	}

	client.mu.RLock()
	count := len(client.activeSubscriptions["userEvents"])
	client.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 active userEvents subscription, got %d", count)
	}

}

// ===== Add/Remove Subscription Tests =====

func TestUnsubscribe(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Subscribe to multiple coins
	msgChan1 := make(chan L2BookMessage)
	sub1, err := client.SubscribeL2Book(context.Background(), "BTC", msgChan1)
	if err != nil {
		t.Fatalf("SubscribeL2Book BTC #1 failed: %v", err)
	}

	msgChan2 := make(chan L2BookMessage)
	sub2, err := client.SubscribeL2Book(context.Background(), "BTC", msgChan2)
	if err != nil {
		t.Fatalf("SubscribeL2Book BTC #2 failed: %v", err)
	}

	msgChan3 := make(chan L2BookMessage)
	sub3, err := client.SubscribeL2Book(context.Background(), "ETH", msgChan3)
	if err != nil {
		t.Fatalf("SubscribeL2Book ETH failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Check initial state
	client.mu.RLock()
	btcSubs := len(client.activeSubscriptions["l2Book:btc"])
	client.mu.RUnlock()

	if btcSubs != 2 {
		t.Errorf("expected 2 BTC subscriptions, got %d", btcSubs)
	}

	// Unsubscribe from one BTC subscription
	sub1.Unsubscribe()

	time.Sleep(50 * time.Millisecond)

	// Check that one was removed
	client.mu.RLock()
	btcSubs = len(client.activeSubscriptions["l2Book:btc"])
	client.mu.RUnlock()

	if btcSubs != 1 {
		t.Errorf(
			"expected 1 BTC subscription after unsubscribe, got %d",
			btcSubs,
		)
	}

	// ETH should be unaffected
	client.mu.RLock()
	ethSubs := len(client.activeSubscriptions["l2Book:eth"])
	client.mu.RUnlock()

	if ethSubs != 1 {
		t.Errorf("expected 1 ETH subscription, got %d", ethSubs)
	}

	sub2.Unsubscribe()
	sub3.Unsubscribe()
	client.Close()
}

// ===== Multiple Subscriptions Per Channel =====

func TestMultipleSubscriptionsPerChannel(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client.Start(ctx)

	// Create subscriber channels
	msgChan1 := make(chan L2BookMessage)
	msgChan2 := make(chan L2BookMessage)

	// Subscribe to the same coin twice
	sub1, err := client.SubscribeL2Book(ctx, "BTC", msgChan1)
	if err != nil {
		t.Fatal(err)
	}
	defer sub1.Unsubscribe()

	sub2, err := client.SubscribeL2Book(ctx, "BTC", msgChan2)
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Unsubscribe()

	time.Sleep(50 * time.Millisecond)

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
	client.handleMessage(msgBytes)

	// Both channels should receive the message concurrently
	received1 := false
	received2 := false

	timeout := time.After(100 * time.Millisecond)
	for i := range 2 {
		select {
		case <-msgChan1:
			received1 = true
		case <-msgChan2:
			received2 = true
		case <-timeout:
			t.Errorf("timeout waiting for message (%d of 2)", i+1)
		}
	}

	if !received1 || !received2 {
		t.Errorf(
			"both subscriptions should receive message: received1=%v, received2=%v",
			received1,
			received2,
		)
	}
}

// ===== Edge Cases =====

func TestEmptyTradesMessage(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	msgChan := make(chan TradesMessage)
	sub, err := client.SubscribeTrades(context.Background(), "ETH", msgChan)
	if err != nil {
		t.Fatalf("SubscribeTrades() failed: %v", err)
	}
	defer sub.Unsubscribe()

	time.Sleep(100 * time.Millisecond)

	// Send empty trades list - should not send to channel
	msgData := map[string]any{
		"channel": "trades",
		"data":    []any{},
	}
	msgBytes, _ := json.Marshal(msgData)
	client.handleMessage(msgBytes)

	// Channel should not receive anything
	select {
	case <-msgChan:
		t.Error("expected no message for empty trades")
	case <-time.After(100 * time.Millisecond):
		// This is expected - no message for empty trades
	}

	client.Close()
}

func TestMissingDataField(t *testing.T) {
	t.Parallel()
	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Subscribe but don't expect message
	msgChan := make(chan L2BookMessage)
	sub, err := client.SubscribeL2Book(context.Background(), "BTC", msgChan)
	if err != nil {
		t.Fatalf("SubscribeL2Book() failed: %v", err)
	}
	defer sub.Unsubscribe()

	time.Sleep(100 * time.Millisecond)

	// Message missing data field - should not crash
	msgData := map[string]any{
		"channel": "l2Book",
	}
	msgBytes, _ := json.Marshal(msgData)
	client.handleMessage(msgBytes)

	// Channel should not receive anything
	select {
	case <-msgChan:
		t.Error("expected no message for malformed message")
	case <-time.After(100 * time.Millisecond):
		// This is expected - no message for malformed data
	}

	client.Close()
}

func TestSubscriptionPayload(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		sub          SubscriptionType
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
			name: "ActiveAssetData includes type, coin, and user",
			sub: ActiveAssetDataSubscription{
				Coin: "SOL",
				User: "0xABC",
			},
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
