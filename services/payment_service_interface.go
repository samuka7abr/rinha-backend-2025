package services

import "my-rinha-go/models"

type PaymentServiceInterface interface {
	ProcessPayment(req models.PaymentRequest) (*models.PaymentRecord, error)
}
