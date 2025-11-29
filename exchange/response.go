package exchange

import (
	"encoding/json"
	"fmt"

	"github.com/banky/go-hyperliquid/types"
)

// Response is a generic top-level response that can hold any "ok" payload type.
type Response[T any] struct {
	Status       string
	Data         *T     // present when Status == "ok"
	ErrorMessage string // present when Status == "err"
}

// wire-level shape:
//
//	{
//	  "status": "ok" | "err",
//	  "response": <object or string>
//	}
type rawResponse struct {
	Status   string          `json:"status"`
	Response json.RawMessage `json:"response"`
}

// UnmarshalJSON lets Response[T] handle both "ok" (object) and "err" (string)
// using the generic type parameter T for the "ok" payload.
func (r *Response[T]) UnmarshalJSON(data []byte) error {
	var raw rawResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal raw response: %w", err)
	}

	r.Status = raw.Status
	r.Data = nil
	r.ErrorMessage = ""

	switch raw.Status {
	case "ok":
		var payload T
		if err := json.Unmarshal(raw.Response, &payload); err != nil {
			return fmt.Errorf("unmarshal ok response body: %w", err)
		}
		r.Data = &payload

	case "err":
		var msg string
		if err := json.Unmarshal(raw.Response, &msg); err != nil {
			return fmt.Errorf("unmarshal error response body: %w", err)
		}
		r.ErrorMessage = msg

	default:
		// Optional: treat unknown status as an error, stash raw JSON as message
		var msg string
		if err := json.Unmarshal(raw.Response, &msg); err != nil {
			msg = string(raw.Response)
		}
		r.ErrorMessage = msg
	}

	return nil
}

// Convenience helpers.

func (r Response[T]) IsOK() bool {
	return r.Status == "ok" && r.Data != nil
}

func (r Response[T]) IsErr() bool {
	return r.Status == "err"
}

// extractStatuses is a generic helper that extracts the statuses slice from the
// raw wire format response containing Type and Data fields.
func extractStatuses[T any](data []byte) ([]T, error) {
	var raw struct {
		Type string          `json:"type"`
		Data ResponseData[T] `json:"data"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return raw.Data.Statuses, nil
}

type ResponseData[T any] struct {
	Statuses []T `json:"statuses"`
}

/*//////////////////////////////////////////////////////////////
                             ORDER
//////////////////////////////////////////////////////////////*/

type OrderResponse OrderStatus

// OrderResponse is a slice of OrderStatus for convenient access without
// needing to access Data.Statuses
type BulkOrdersResponse []OrderStatus

// UnmarshalJSON unmarshals the response into a flat slice of OrderStatus
func (or *BulkOrdersResponse) UnmarshalJSON(data []byte) error {
	statuses, err := extractStatuses[OrderStatus](data)
	if err != nil {
		return fmt.Errorf("unmarshal order response: %w", err)
	}
	*or = BulkOrdersResponse(statuses)
	return nil
}

type OrderStatus struct {
	Resting *OrderStatusResting `json:"resting,omitempty"`
	Filled  *OrderStatusFilled  `json:"filled,omitempty"`
	Error   *string             `json:"error,omitempty"`
}

type OrderStatusResting struct {
	Oid      int64        `json:"oid"`
	ClientId *types.Cloid `json:"cloid"`
	Status   string       `json:"status"`
}

type OrderStatusFilled struct {
	TotalSz string `json:"totalSz"`
	AvgPx   string `json:"avgPx"`
	Oid     int64  `json:"oid"`
}

/*//////////////////////////////////////////////////////////////
                             CLOSE
//////////////////////////////////////////////////////////////*/

// CancelResponse is a slice of CloseStatus for convenient access without
// needing to access Data.Statuses
type CancelResponse []CloseStatus

// UnmarshalJSON unmarshals the response into a flat slice of CloseStatus
func (cr *CancelResponse) UnmarshalJSON(data []byte) error {
	statuses, err := extractStatuses[CloseStatus](data)
	if err != nil {
		return fmt.Errorf("unmarshal cancel response: %w", err)
	}
	*cr = CancelResponse(statuses)
	return nil
}

// TODO: Fix close error state
type CloseStatus string

/*//////////////////////////////////////////////////////////////
                             MODIFY
//////////////////////////////////////////////////////////////*/

// ModifyResponse is a slice of OrderStatus for convenient access without
// needing to access Data.Statuses
type ModifyResponse []OrderStatus

// UnmarshalJSON unmarshals the response into a flat slice of OrderStatus
func (mr *ModifyResponse) UnmarshalJSON(data []byte) error {
	statuses, err := extractStatuses[OrderStatus](data)
	if err != nil {
		return fmt.Errorf("unmarshal modify response: %w", err)
	}
	*mr = ModifyResponse(statuses)
	return nil
}
