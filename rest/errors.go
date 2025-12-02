package rest

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
)

type ClientError struct {
	StatusCode int64
	Code       string
	Msg        string
	Headers    http.Header
	Data       any
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("client error (status %d): %s", e.StatusCode, e.Msg)
}

type ServerError struct {
	StatusCode int64
	Text       string
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("server error (status %d): %s", e.StatusCode, e.Text)
}

type errorResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func handleException(resp *resty.Response) error {
	statusCode := int64(resp.StatusCode())

	if statusCode < 400 {
		return nil
	}

	if statusCode >= 400 && statusCode < 500 {
		var errResp errorResponse
		err := json.Unmarshal(resp.Body(), &errResp)

		if err != nil {
			return &ClientError{
				StatusCode: statusCode,
				Code:       "",
				Msg:        string(resp.Body()),
				Headers:    resp.Header(),
				Data:       nil,
			}
		}

		if errResp.Code == "" && errResp.Msg == "" {
			return &ClientError{
				StatusCode: statusCode,
				Code:       "",
				Msg:        string(resp.Body()),
				Headers:    resp.Header(),
				Data:       nil,
			}
		}

		return &ClientError{
			StatusCode: statusCode,
			Code:       errResp.Code,
			Msg:        errResp.Msg,
			Headers:    resp.Header(),
			Data:       errResp.Data,
		}
	}

	return &ServerError{
		StatusCode: statusCode,
		Text:       string(resp.Body()),
	}
}
