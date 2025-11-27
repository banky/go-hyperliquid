package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

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
	case "subscriptionResponse":
		// Don't care about these
		break
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

	routeMessage(m, "allMids", msg)
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
	routeMessage(m, identifier, msg)
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
	routeMessage(m, identifier, msg)
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

	routeMessage(m, "userEvents", msg)
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
	routeMessage(m, identifier, msg)
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
	routeMessage(m, identifier, msg)
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
	routeMessage(m, identifier, msg)
}

func (m *Client) handleOrderUpdates(raw map[string]any, identifier string) {
	dataRaw, ok := raw["data"]
	if !ok {
		return
	}

	msg := OrderUpdatesMessage(dataRaw.(map[string]any))
	routeMessage(m, identifier, msg)
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
		routeMessage(m, identifier, msg)
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
		identifier := fmt.Sprintf(
			"userNonFundingLedgerUpdates:%s",
			strings.ToLower(user),
		)
		routeMessage(m, identifier, msg)
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
		routeMessage(m, identifier, msg)
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
			log.Printf(
				"failed to unmarshal activeSpotAssetCtx message: %v",
				err,
			)
			return
		}
		identifier := fmt.Sprintf(
			"activeAssetCtx:%s",
			strings.ToLower(msg.Coin),
		)
		routeMessage(m, identifier, msg)
	} else {
		var msg ActiveAssetCtxMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Printf("failed to unmarshal activeAssetCtx message: %v", err)
			return
		}
		identifier := fmt.Sprintf("activeAssetCtx:%s", strings.ToLower(msg.Coin))
		routeMessage(m, identifier, msg)
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

	identifier := fmt.Sprintf(
		"activeAssetData:%s,%s",
		strings.ToLower(msg.Coin),
		strings.ToLower(msg.User),
	)
	routeMessage(m, identifier, msg)
}

// routeMessage routes a message to all subscriptions registered for that identifier
func routeMessage[T any](m *Client, identifier string, msg T) {
	m.mu.RLock()
	subscriptions := m.activeSubscriptions[identifier]
	m.mu.RUnlock()

	if len(subscriptions) == 0 {
		log.Printf(
			"websocket message from unexpected subscription: %s",
			identifier,
		)
		return
	}

	for _, sub := range subscriptions {
		ch, ok := sub.internalChan.(chan T)
		if !ok {
			panic(
				fmt.Sprintf(
					"subscription internal channel has wrong type for %s (id: %d)",
					identifier,
					sub.id,
				),
			)
		}

		ch <- msg
	}
}
