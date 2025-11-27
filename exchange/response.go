package exchange

type Response struct {
	Status   string          `json:"status"`
	Response ResponseWrapper `json:"response"`
}

type ResponseWrapper struct {
	Type string       `json:"type"`
	Data ResponseData `json:"data"`
}

type ResponseData struct {
	Statuses []OrderStatus `json:"statuses"`
}

type OrderStatus struct {
	Resting *RestingStatus `json:"resting,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type RestingStatus struct {
	OID int64 `json:"oid"`
}
