package exchange

import (
	"encoding/json"
	"fmt"

	"github.com/banky/go-hyperliquid/types"
)

// response is a generic top-level response that can hold any "ok" payload type.
type response[T any] struct {
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
func (r *response[T]) UnmarshalJSON(data []byte) error {
	var raw rawResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Status = raw.Status
	r.Data = nil
	r.ErrorMessage = ""

	switch raw.Status {
	case "ok":
		var payload T
		if err := json.Unmarshal(raw.Response, &payload); err != nil {
			return err
		}
		r.Data = &payload

	case "err":
		var msg string
		if err := json.Unmarshal(raw.Response, &msg); err != nil {
			return err
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

func (r response[T]) IsOK() bool {
	return r.Status == "ok" && r.Data != nil
}

func (r response[T]) IsErr() bool {
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
		return nil, err
	}

	return raw.Data.Statuses, nil
}

type ResponseData[T any] struct {
	Statuses []T `json:"statuses"`
}

/*//////////////////////////////////////////////////////////////
                             ORDER
//////////////////////////////////////////////////////////////*/

// OrderResponse represents the status of an order
// It can contain either resting or filled information, or will error if the
// order failed
type OrderResponse struct {
	Resting *OrderStatusResting `json:"resting,omitempty"`
	Filled  *OrderStatusFilled  `json:"filled,omitempty"`
}

// OrderResponse is a slice of OrderStatus for convenient access without
// needing to access Data.Statuses
type BulkOrdersResponse []OrderResponse

// UnmarshalJSON unmarshals the response into a flat slice of OrderStatus
func (or *BulkOrdersResponse) UnmarshalJSON(data []byte) error {
	statuses, err := extractStatuses[OrderResponse](data)
	if err != nil {
		return err
	}
	*or = BulkOrdersResponse(statuses)
	return nil
}

// UnmarshalJSON handles errors by bubbling them up
func (os *OrderResponse) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as an object with resting/filled/error fields
	var obj struct {
		Resting *OrderStatusResting `json:"resting,omitempty"`
		Filled  *OrderStatusFilled  `json:"filled,omitempty"`
		Error   *string             `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// If there's an error in the response, bubble it up
	if obj.Error != nil {
		return fmt.Errorf("%s", *obj.Error)
	}

	os.Resting = obj.Resting
	os.Filled = obj.Filled
	return nil
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
                             CANCEL
//////////////////////////////////////////////////////////////*/

type CancelResponse struct {
	Status string `json:"status"`
}

type BulkCancelResponse []CancelResponse

// UnmarshalJSON unmarshals the response into a flat slice of CloseStatus
func (cr *BulkCancelResponse) UnmarshalJSON(data []byte) error {
	statuses, err := extractStatuses[CancelResponse](data)
	if err != nil {
		return err
	}
	*cr = BulkCancelResponse(statuses)
	return nil
}

// UnmarshalJSON handles both string and object formats for CloseStatus
// If an error object is received, it returns an error instead of storing it
func (c *CancelResponse) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as a string first (e.g., "success")
	var statusStr string
	if err := json.Unmarshal(data, &statusStr); err == nil {
		c.Status = statusStr
		return nil
	}

	// Fall back to unmarshaling as an object with error field
	var obj struct {
		Error *string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// If there's an error in the response, bubble it up
	if obj.Error != nil {
		return fmt.Errorf("%s", *obj.Error)
	}

	c.Status = ""
	return nil
}

/*//////////////////////////////////////////////////////////////
                            UPDATES
//////////////////////////////////////////////////////////////*/

type UpdateResponse struct {
	Type string `json:"type"`
}

type SetReferrerResponse struct {
	Status string `json:"status"`
}

type CreateSubAccountResponse struct {
	Status string `json:"status"`
}
