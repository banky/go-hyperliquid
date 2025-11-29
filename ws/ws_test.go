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
	"github.com/ethereum/go-ethereum/common"
	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
)

// ===== Suite wiring =====

type WSSuite struct{}

func TestWSSuite(t *testing.T) {
	tdsuite.Run(t, &WSSuite{})
}

// ===== Subscription Identifier Tests =====

func (s *WSSuite) TestSubscriptionIdentifiers(assert, require *td.T) {
	require.Parallel()

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
			name: "UserEvents",
			sub: UserEventsSubscription{
				User: common.HexToAddress("0xABC"),
			},
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
		got := tt.sub.identifier()
		require.Cmp(got, tt.expectedID, tt.name)
	}
}

// ===== Mock WebSocket Server =====

// mockWSServer simulates a Hyperliquid WebSocket server
type mockWSServer struct {
	server *httptest.Server
	url    string
}

func newMockWSServer(t testing.TB) *mockWSServer {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Logf("websocket accept error: %v", err)
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "test complete")

			// Send connection established message
			_ = conn.Write(
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
					_ = conn.Write(
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

func (s *WSSuite) TestClientStartStop(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	// Give it time to process the connection message
	time.Sleep(100 * time.Millisecond)

	client.Close()
}

// ===== Channel-Based Subscription Tests =====

func (s *WSSuite) TestChannelSubscription(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	time.Sleep(100 * time.Millisecond)

	// Subscribe to AllMids with a channel
	msgChan := make(chan AllMidsMessage)
	sub, err := client.SubscribeAllMids(ctx, msgChan)
	require.CmpNoError(err)
	require.NotNil(sub, "expected non-nil subscription")

	time.Sleep(100 * time.Millisecond)

	// Check that subscription is active
	client.mu.RLock()
	active := len(client.activeSubscriptions["allMids"])
	client.mu.RUnlock()
	require.Cmp(active, 1, "expected 1 active allMids subscription")

	// Unsubscribe
	sub.Unsubscribe()

	time.Sleep(50 * time.Millisecond)

	// Check that subscription is gone
	client.mu.RLock()
	active = len(client.activeSubscriptions["allMids"])
	client.mu.RUnlock()
	require.Cmp(
		active,
		0,
		"expected 0 active allMids subscriptions after unsubscribe",
	)

	client.Close()
}

// ===== Message Routing Tests =====

func (s *WSSuite) TestL2BookMessageRouting(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	// Create subscriber channel
	msgChan := make(chan L2BookMessage)
	sub, err := client.SubscribeL2Book(ctx, "BTC", msgChan)
	require.CmpNoError(err)
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
		require.Cmp(received.Coin, "BTC")
		require.Cmp(received.Time, int64(1234567890))
	case <-time.After(1 * time.Second):
		require.True(false, "timeout waiting for message")
	}
}

func (s *WSSuite) TestTradesMessageRouting(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	// Create subscriber channel
	msgChan := make(chan TradesMessage)
	sub, err := client.SubscribeTrades(ctx, "ETH", msgChan)
	require.CmpNoError(err)
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
		require.Cmp(len(received.Trades), 2)
		require.Cmp(received.Trades[0].Coin, "ETH")
	case <-time.After(1 * time.Second):
		require.True(false, "timeout waiting for message")
	}
}

// ===== Multiplexing Constraint Tests =====

func (s *WSSuite) TestUserEventsDuplicateSubscription(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	// First subscription should work
	msgChan1 := make(chan UserEventsMessage)
	sub1, err := client.SubscribeUserEvents(
		context.Background(),
		common.HexToAddress("0xABC"),
		msgChan1,
	)
	require.CmpNoError(err, "first SubscribeUserEvents() failed")
	defer sub1.Unsubscribe()

	time.Sleep(100 * time.Millisecond)

	// Second subscription to same channel (userEvents) should fail
	// because userEvents only allows one subscription
	msgChan2 := make(chan UserEventsMessage)
	sub2, err := client.SubscribeUserEvents(
		context.Background(),
		common.HexToAddress("0xDEF"),
		msgChan2,
	)
	if err == nil {
		sub2.Unsubscribe()
		require.True(false, "expected second userEvents subscription to fail")
	}

	client.mu.RLock()
	count := len(client.activeSubscriptions["userEvents"])
	client.mu.RUnlock()

	require.Cmp(count, 1, "expected 1 active userEvents subscription")
}

// ===== Add/Remove Subscription Tests =====

func (s *WSSuite) TestUnsubscribe(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	time.Sleep(100 * time.Millisecond)

	// Subscribe to multiple coins
	msgChan1 := make(chan L2BookMessage)
	sub1, err := client.SubscribeL2Book(context.Background(), "BTC", msgChan1)
	require.CmpNoError(err, "SubscribeL2Book BTC #1 failed")

	msgChan2 := make(chan L2BookMessage)
	sub2, err := client.SubscribeL2Book(context.Background(), "BTC", msgChan2)
	require.CmpNoError(err, "SubscribeL2Book BTC #2 failed")

	msgChan3 := make(chan L2BookMessage)
	sub3, err := client.SubscribeL2Book(context.Background(), "ETH", msgChan3)
	require.CmpNoError(err, "SubscribeL2Book ETH failed")

	time.Sleep(100 * time.Millisecond)

	// Check initial state
	client.mu.RLock()
	btcSubs := len(client.activeSubscriptions["l2Book:btc"])
	client.mu.RUnlock()
	require.Cmp(btcSubs, 2, "expected 2 BTC subscriptions")

	// Unsubscribe from one BTC subscription
	sub1.Unsubscribe()

	time.Sleep(50 * time.Millisecond)

	// Check that one was removed
	client.mu.RLock()
	btcSubs = len(client.activeSubscriptions["l2Book:btc"])
	client.mu.RUnlock()
	require.Cmp(btcSubs, 1, "expected 1 BTC subscription after unsubscribe")

	// ETH should be unaffected
	client.mu.RLock()
	ethSubs := len(client.activeSubscriptions["l2Book:eth"])
	client.mu.RUnlock()
	require.Cmp(ethSubs, 1, "expected 1 ETH subscription")

	sub2.Unsubscribe()
	sub3.Unsubscribe()
	client.Close()
}

// ===== Multiple Subscriptions Per Channel =====

func (s *WSSuite) TestMultipleSubscriptionsPerChannel(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	// Create subscriber channels
	msgChan1 := make(chan L2BookMessage)
	msgChan2 := make(chan L2BookMessage)

	// Subscribe to the same coin twice
	sub1, err := client.SubscribeL2Book(ctx, "BTC", msgChan1)
	require.CmpNoError(err)
	defer sub1.Unsubscribe()

	sub2, err := client.SubscribeL2Book(ctx, "BTC", msgChan2)
	require.CmpNoError(err)
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

	// Both channels should receive the message
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
			require.True(false, "timeout waiting for message (%d of 2)", i+1)
		}
	}

	require.True(
		received1 && received2,
		"both subscriptions should receive message: received1=%v, received2=%v",
		received1,
		received2,
	)
}

// ===== Edge Cases =====

func (s *WSSuite) TestEmptyTradesMessage(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	time.Sleep(100 * time.Millisecond)

	msgChan := make(chan TradesMessage)
	sub, err := client.SubscribeTrades(context.Background(), "ETH", msgChan)
	require.CmpNoError(err, "SubscribeTrades() failed")
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
		require.True(false, "expected no message for empty trades")
	case <-time.After(100 * time.Millisecond):
		// expected - no message
	}

	client.Close()
}

func (s *WSSuite) TestMissingDataField(assert, require *td.T) {
	t := require.TB
	require.Parallel()

	server := newMockWSServer(t)
	defer server.close()

	client := New(server.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	require.CmpNoError(err)

	time.Sleep(100 * time.Millisecond)

	// Subscribe but don't expect message
	msgChan := make(chan L2BookMessage)
	sub, err := client.SubscribeL2Book(context.Background(), "BTC", msgChan)
	require.CmpNoError(err, "SubscribeL2Book() failed")
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
		require.True(false, "expected no message for malformed message")
	case <-time.After(100 * time.Millisecond):
		// expected - no message
	}

	client.Close()
}

// ===== Subscription payload shape =====

func (s *WSSuite) TestSubscriptionPayload(assert, require *td.T) {
	require.Parallel()

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
		payload := tt.sub.subscriptionPayload()
		payloadMap, ok := payload.(map[string]any)
		require.True(
			ok,
			"subscription payload is not a map in test %q",
			tt.name,
		)

		for _, key := range tt.expectedKeys {
			_, exists := payloadMap[key]
			require.True(
				exists,
				"expected key %q in payload for test %q",
				key,
				tt.name,
			)
		}
	}
}
