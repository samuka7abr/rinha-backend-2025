package models

import (
	"time"
)

type PaymentRequest struct {
	CorrelationID string  `json:"correlationId" validate:"required"`
	Amount        float64 `json:"amount" validate:"required,gt=0"`
}

type PaymentProcessorRequest struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

type PaymentProcessorResponse struct {
	Message string `json:"message"`
}

type PaymentRecord struct {
	ID            int       `json:"id"`
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	Processor     string    `json:"processor"`
	ProcessedAt   time.Time `json:"processedAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

type ProcessorSummary struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type PaymentsSummaryResponse struct {
	Default  ProcessorSummary `json:"default"`
	Fallback ProcessorSummary `json:"fallback"`
}
