// Package client provides core functions for
// network requests to Hyperliquid API endpoints
package rest

import (
	"context"
	"encoding/json"
	"time"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/go-resty/resty/v2"
	"github.com/samber/mo"
)

type Client struct {
	baseUrl string
	timeout mo.Option[uint]
}

type Config struct {
	// BaseUrl is the base URL for the Hyperliquid API
	// If none is provided, the mainnet url will be used
	BaseUrl string
	// Timeout is the timeout for network requests
	// If none is provided, no timeout will be enforced
	Timeout uint
}

// New creates a new client instance with the
// provided configuration.
func New(c Config) Client {
	var baseUrl string = c.BaseUrl
	var timeout mo.Option[uint]

	if c.BaseUrl == "" {
		baseUrl = constants.MAINNET_API_URL
	}
	if c.Timeout != 0 {
		timeout = mo.Some(c.Timeout)
	}

	return Client{
		baseUrl: baseUrl,
		timeout: timeout,
	}
}

// Post sends a POST request to the specified path with the provided body.
func Post[Resp any](client Client, path string, body any) (Resp, error) {
	var result Resp

	r := resty.
		New().
		SetJSONMarshaler(json.Marshal).
		SetJSONUnmarshaler(json.Unmarshal)

	url := client.baseUrl + path

	// Create context with timeout if specified
	ctx := context.Background()
	if timeout, ok := client.timeout.Get(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	resp, err := r.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetResult(&result).
		Post(url)

	if err != nil {
		return result, err
	}

	if err := handleException(resp); err != nil {
		return result, err
	}

	return result, nil
}
