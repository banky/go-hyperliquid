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

// ClientInterface defines the contract for REST API calls
type ClientInterface interface {
	Post(ctx context.Context, path string, body any, result any) error
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
func New(c Config) *Client {
	var baseUrl string = c.BaseUrl
	var timeout mo.Option[uint]

	if c.BaseUrl == "" {
		baseUrl = constants.MAINNET_API_URL
	}
	if c.Timeout != 0 {
		timeout = mo.Some(c.Timeout)
	}

	client := &Client{
		baseUrl: baseUrl,
		timeout: timeout,
	}

	return client
}

// Post sends a POST request to the specified path with the provided body.
func (c *Client) Post(
	ctx context.Context,
	path string,
	body any,
	result any,
) error {
	r := resty.
		New().
		SetJSONMarshaler(json.Marshal).
		SetJSONUnmarshaler(json.Unmarshal)

	url := c.baseUrl + path

	// Apply timeout to context if specified
	if timeout, ok := c.timeout.Get(); ok {
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
		return err
	}

	if err := handleException(resp); err != nil {
		return err
	}

	return nil
}
