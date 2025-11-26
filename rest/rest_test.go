package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testRequest struct {
	Name string `json:"name"`
}

type testResponse struct {
	Status string `json:"status"`
	Value  int    `json:"value"`
}

func TestPostSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testResponse{Status: "ok", Value: 42})
	}))
	defer server.Close()

	client := New(Config{BaseUrl: server.URL})
	result, err := Post[testResponse](client, "/test", testRequest{Name: "test"})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != "ok" || result.Value != 42 {
		t.Errorf("expected {ok 42}, got {%s %d}", result.Status, result.Value)
	}
}

func TestPostClientErrorWithJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"code": "INVALID_REQUEST",
			"msg":  "Request validation failed",
			"data": map[string]string{"field": "name"},
		})
	}))
	defer server.Close()

	client := New(Config{BaseUrl: server.URL})
	_, err := Post[testResponse](client, "/test", testRequest{Name: ""})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("expected ClientError, got %T", err)
	}

	if clientErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", clientErr.StatusCode)
	}

	if clientErr.Code != "INVALID_REQUEST" {
		t.Errorf("expected code INVALID_REQUEST, got %s", clientErr.Code)
	}

	if clientErr.Msg != "Request validation failed" {
		t.Errorf("expected msg 'Request validation failed', got %s", clientErr.Msg)
	}

	if clientErr.Data == nil {
		t.Error("expected data to be populated")
	}
}

func TestPostClientErrorWithoutJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	client := New(Config{BaseUrl: server.URL})
	_, err := Post[testResponse](client, "/test", testRequest{Name: "test"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("expected ClientError, got %T", err)
	}

	if clientErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", clientErr.StatusCode)
	}

	if clientErr.Msg != "Unauthorized" {
		t.Errorf("expected msg 'Unauthorized', got %s", clientErr.Msg)
	}
}

func TestPostServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := New(Config{BaseUrl: server.URL})
	_, err := Post[testResponse](client, "/test", testRequest{Name: "test"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	serverErr, ok := err.(*ServerError)
	if !ok {
		t.Fatalf("expected ServerError, got %T", err)
	}

	if serverErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", serverErr.StatusCode)
	}

	if serverErr.Text != "Internal Server Error" {
		t.Errorf("expected text 'Internal Server Error', got %s", serverErr.Text)
	}
}

func TestPostWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testResponse{Status: "ok", Value: 42})
	}))
	defer server.Close()

	// Create client with 5 second timeout (more than enough for fast server)
	client := New(Config{BaseUrl: server.URL, Timeout: 5})
	result, err := Post[testResponse](client, "/test", testRequest{Name: "test"})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != "ok" || result.Value != 42 {
		t.Errorf("expected {ok 42}, got {%s %d}", result.Status, result.Value)
	}
}
