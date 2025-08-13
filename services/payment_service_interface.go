package services

import (
	"my-rinha-go/models"
	"time"
)

type PaymentServiceInterface interface {
	ProcessPayment(req models.PaymentRequest) (*models.PaymentRecord, error)
	GetPaymentsSummary(from, to *time.Time) (*models.PaymentsSummaryResponse, error)
}
