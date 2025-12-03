package exchange

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	okRestingJSON = `
{
   "status":"ok",
   "response":{
      "type":"order",
      "data":{
         "statuses":[
            {
               "resting":{
                  "oid":77738308
               }
            }
         ]
      }
   }
}`

	okErrorStatusJSON = `
{
   "status":"ok",
   "response":{
      "type":"order",
      "data":{
         "statuses":[
            {
               "error":"Order must have minimum value of $10."
            }
         ]
      }
   }
}`

	errTopLevelJSON = `
{
   "status": "err",
   "response": "User or API Wallet 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 does not exist."
}`
)

func TestUnmarshalResponse_OK_RestingStatus(t *testing.T) {
	var resp response[BulkOrdersResponse]

	if err := json.Unmarshal([]byte(okRestingJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshalling okRestingJSON: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("expected Status == %q, got %q", "ok", resp.Status)
	}

	if resp.Data == nil {
		t.Fatalf("expected Data to be non-nil for ok response")
	}

	if resp.ErrorMessage != "" {
		t.Fatalf(
			"expected ErrorMessage to be empty for ok response, got %q",
			resp.ErrorMessage,
		)
	}

	if len(*resp.Data) != 1 {
		t.Fatalf("expected 1 status, got %d", len(*resp.Data))
	}

	status := (*resp.Data)[0]
	if status.Resting == nil {
		t.Fatalf("expected Resting to be non-nil")
	}

	const expectedOID int64 = 77738308
	if status.Resting.Oid != expectedOID {
		t.Fatalf(
			"expected Resting.OID == %d, got %d",
			expectedOID,
			status.Resting.Oid,
		)
	}
}

func TestUnmarshalArrIntoSingleOrdersResponse(t *testing.T) {
	var resp response[OrderResponse]

	if err := json.Unmarshal([]byte(okRestingJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshalling okRestingJSON: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("expected Status == %q, got %q", "ok", resp.Status)
	}

	if resp.Data == nil {
		t.Fatalf("expected Data to be non-nil for ok response")
	}

	if resp.ErrorMessage != "" {
		t.Fatalf(
			"expected ErrorMessage to be empty for ok response, got %q",
			resp.ErrorMessage,
		)
	}

	// if len(*resp.Data) != 1 {
	// 	t.Fatalf("expected 1 status, got %d", len(*resp.Data))
	// }

	status := (*resp.Data)
	if status.Resting == nil {
		t.Fatalf("expected Resting to be non-nil")
	}

	const expectedOID int64 = 77738308
	if status.Resting.Oid != expectedOID {
		t.Fatalf(
			"expected Resting.OID == %d, got %d",
			expectedOID,
			status.Resting.Oid,
		)
	}
}

func TestUnmarshalResponse_OK_ErrorStatus(t *testing.T) {
	var resp response[BulkOrdersResponse]

	err := json.Unmarshal([]byte(okErrorStatusJSON), &resp)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Order must have minimum value of $10.") {
		t.Fatal("Error doesn't contain expected message")
	}
}

func TestUnmarshalResponse_Err_TopLevel(t *testing.T) {
	var resp response[OrderResponse]

	if err := json.Unmarshal([]byte(errTopLevelJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshalling errTopLevelJSON: %v", err)
	}

	if resp.Status != "err" {
		t.Fatalf("expected Status == %q, got %q", "err", resp.Status)
	}

	if resp.Data != nil {
		t.Fatalf("expected Data to be nil for err response, got %+v", resp.Data)
	}

	expectedMsg := "User or API Wallet 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 does not exist."
	if resp.ErrorMessage != expectedMsg {
		t.Fatalf(
			"expected ErrorMessage == %q, got %q",
			expectedMsg,
			resp.ErrorMessage,
		)
	}
}
