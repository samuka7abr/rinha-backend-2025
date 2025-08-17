package models

type Payment struct {
	Amount int64 `json:"amount"`
	Ts     int64 `json:"ts,omitempty"` 
}
