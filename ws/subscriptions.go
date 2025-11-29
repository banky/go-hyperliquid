package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// ===== Type-safe subscription methods =====

// SubscribeAllMids subscribes to all mid-prices
func (m *Client) SubscribeAllMids(
	ctx context.Context,
	ch chan<- AllMidsMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, AllMidsSubscription{}, ch)
}

// SubscribeL2Book subscribes to level 2 order book for a coin
func (m *Client) SubscribeL2Book(
	ctx context.Context,
	coin string,
	ch chan<- L2BookMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, L2BookSubscription{Coin: coin}, ch)
}

// SubscribeTrades subscribes to trades for a coin
func (m *Client) SubscribeTrades(
	ctx context.Context,
	coin string,
	ch chan<- TradesMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, TradesSubscription{Coin: coin}, ch)
}

// SubscribeUserEvents subscribes to user events
func (m *Client) SubscribeUserEvents(
	ctx context.Context,
	user string,
	ch chan<- UserEventsMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, UserEventsSubscription{User: user}, ch)
}

// SubscribeUserFills subscribes to user fills
func (m *Client) SubscribeUserFills(
	ctx context.Context,
	user string,
	ch chan<- UserFillsMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, UserFillsSubscription{User: user}, ch)
}

// SubscribeCandle subscribes to candle data
func (m *Client) SubscribeCandle(
	ctx context.Context,
	coin string,
	interval string,
	ch chan<- CandleMessage,
) (Subscription, error) {
	return newWSSubscription(
		ctx,
		m,
		CandleSubscription{Coin: coin, Interval: interval},
		ch,
	)
}

// SubscribeOrderUpdates subscribes to order updates
func (m *Client) SubscribeOrderUpdates(
	ctx context.Context,
	user string,
	ch chan<- OrderUpdatesMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, OrderUpdatesSubscription{User: user}, ch)
}

// SubscribeUserFundings subscribes to user fundings
func (m *Client) SubscribeUserFundings(
	ctx context.Context,
	user string,
	ch chan<- UserFundingsMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, UserFundingsSubscription{User: user}, ch)
}

// SubscribeUserNonFundingLedgerUpdates subscribes to non-funding ledger updates
func (m *Client) SubscribeUserNonFundingLedgerUpdates(
	ctx context.Context,
	user string,
	ch chan<- UserNonFundingLedgerUpdatesMessage,
) (Subscription, error) {
	return newWSSubscription(
		ctx,
		m,
		UserNonFundingLedgerUpdatesSubscription{User: user},
		ch,
	)
}

// SubscribeWebData2 subscribes to web data
func (m *Client) SubscribeWebData2(
	ctx context.Context,
	user string,
	ch chan<- WebData2Message,
) (Subscription, error) {
	return newWSSubscription(ctx, m, WebData2Subscription{User: user}, ch)
}

// SubscribeBbo subscribes to best bid/offer data
func (m *Client) SubscribeBbo(
	ctx context.Context,
	coin string,
	ch chan<- BboMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, BboSubscription{Coin: coin}, ch)
}

// SubscribeActiveAssetCtx subscribes to active asset context
func (m *Client) SubscribeActiveAssetCtx(
	ctx context.Context,
	coin string,
	ch chan<- ActiveAssetCtxMessage,
) (Subscription, error) {
	return newWSSubscription(ctx, m, ActiveAssetCtxSubscription{Coin: coin}, ch)
}

// SubscribeActiveAssetData subscribes to active asset data
func (m *Client) SubscribeActiveAssetData(
	ctx context.Context,
	coin string,
	user string,
	ch chan<- ActiveAssetDataMessage,
) (Subscription, error) {
	return newWSSubscription(
		ctx,
		m,
		ActiveAssetDataSubscription{Coin: coin, User: user},
		ch,
	)
}

// newWSSubscription sets up a websocket subscription, wires it to ctx,
// and returns a Subscription. It centralizes error-channel and goroutine logic.
func newWSSubscription[T any](
	ctx context.Context,
	m *Client,
	sub SubscriptionType,
	ch chan<- T,
) (Subscription, error) {
	// Derived context that represents the lifetime of this subscription.
	subCtx, cancel := context.WithCancel(ctx)

	errChan := make(chan error, 1)
	id := m.nextSubscriptionID()

	// Register with the remote WS + internal maps.
	if err := subscribe(m, sub, ch, id); err != nil {
		cancel()
		close(errChan)
		return nil, err
	}

	s := &subscription{
		cancel:  cancel,
		errChan: errChan,
	}

	// Single owner of errChan and of unsubscribeInternal cleanup.
	go func() {
		<-subCtx.Done()

		// Best-effort send of the terminal error; non-blocking.
		select {
		case errChan <- subCtx.Err():
		default:
		}

		close(errChan)

		// Remove from client's subscription map.
		unsubscribeInternal[T](m, sub, id)
	}()

	return s, nil
}

// nextSubscriptionID increments and returns a unique subscription ID.
func (m *Client) nextSubscriptionID() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptionIDCounter++
	return m.subscriptionIDCounter
}

func subscribe[T any](
	m *Client,
	sub SubscriptionType,
	subscriberChan chan<- T,
	id int64,
) error {
	identifier := sub.identifier()
	internalChan := make(chan T)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate restrictions
	if identifier == "userEvents" || identifier == "orderUpdates" {
		if len(m.activeSubscriptions[identifier]) != 0 {
			return fmt.Errorf(
				"cannot subscribe to %s multiple times",
				identifier,
			)
		}
	}

	// Add to active subscriptions
	m.activeSubscriptions[identifier] = append(
		m.activeSubscriptions[identifier],
		&channelSubscription{
			internalChan: internalChan,
			id:           id,
		},
	)

	// Launch delivery goroutine that forwards from internal channel to
	// subscriber channel
	go deliveryLoop(internalChan, subscriberChan)

	// Send subscription message to server (if connected)
	if m.conn != nil {
		msg := map[string]any{
			"method":       "subscribe",
			"subscription": sub.subscriptionPayload(),
		}
		data, _ := json.Marshal(msg)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.conn.Write(ctx, websocket.MessageText, data)
	}

	return nil

}

func deliveryLoop[T any](
	internalChan chan T,
	subscriberChan chan<- T,
) {
	for msg := range internalChan {
		subscriberChan <- msg
	}
}

// unsubscribeInternal removes a subscription and closes its internal channel
func unsubscribeInternal[T any](
	m *Client,
	sub SubscriptionType,
	subscriptionID int64,
) bool {
	m.mu.Lock()
	identifier := sub.identifier()
	activeSubscriptions := m.activeSubscriptions[identifier]

	// Find and close the internal channel
	var internalChan chan T
	newActiveSubscriptions := make([]*channelSubscription, 0)
	for _, s := range activeSubscriptions {
		if s.id == subscriptionID {
			i, ok := s.internalChan.(chan T)
			if !ok {
				panic(
					fmt.Sprintf(
						"subscription internal channel in unsubscribe has wrong type for %s (id: %d)",
						identifier,
						s.id,
					),
				)
			}
			internalChan = i
		} else {
			newActiveSubscriptions = append(newActiveSubscriptions, s)
		}
	}

	if internalChan != nil {
		close(internalChan)
	}

	// If no more subscriptions for this identifier, send unsubscribe (if
	// connected)
	if len(newActiveSubscriptions) == 0 && m.conn != nil {
		msg := map[string]any{
			"method":       "unsubscribe",
			"subscription": sub.subscriptionPayload(),
		}
		data, _ := json.Marshal(msg)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		m.mu.Unlock()
		defer cancel()
		err := m.conn.Write(ctx, websocket.MessageText, data)
		if err != nil {
			// Ignore errors that are clearly “connection is gone”
			if strings.Contains(
				err.Error(),
				"use of closed network connection",
			) ||
				websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				// maybe log at debug-level if you have such a thing
			} else {
				log.Printf("error sending unsubscribe message: %v\n", err)
			}
		}
		m.mu.Lock()
	}

	m.activeSubscriptions[identifier] = newActiveSubscriptions
	m.mu.Unlock()

	return internalChan != nil
}
