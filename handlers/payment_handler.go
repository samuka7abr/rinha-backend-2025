package handlers

import (
	"encoding/json"
	"net/http"

	"my-rinha-go/models"
	"my-rinha-go/services"

	"github.com/sirupsen/logrus"
)

type PaymentHandler struct {
	paymentService services.PaymentServiceInterface
	logger         *logrus.Logger
}

func NewPaymentHandler(paymentService services.PaymentServiceInterface) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		logger:         logrus.New(),
	}
}

func (ph *PaymentHandler) SetPaymentService(service services.PaymentServiceInterface) {
	ph.paymentService = service
}

func (ph *PaymentHandler) PostPayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ph.writeErrorResponse(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}

	var req models.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ph.logger.WithError(err).Warn("Erro ao decodificar JSON da requisição")
		ph.writeErrorResponse(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	if req.CorrelationID == "" {
		ph.writeErrorResponse(w, http.StatusBadRequest, "correlationId é obrigatório")
		return
	}

	if req.Amount <= 0 {
		ph.writeErrorResponse(w, http.StatusBadRequest, "amount deve ser maior que 0")
		return
	}

	record, err := ph.paymentService.ProcessPayment(req)
	if err != nil {
		ph.logger.WithError(err).Error("Erro ao processar pagamento")
		ph.writeErrorResponse(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}

	ph.logger.WithFields(logrus.Fields{
		"correlationId": req.CorrelationID,
		"amount":        req.Amount,
		"processor":     record.Processor,
	}).Info("Pagamento processado com sucesso")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status":  "success",
		"message": "Pagamento processado com sucesso",
	}
	json.NewEncoder(w).Encode(response)
}

func (ph *PaymentHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error": message,
	}
	json.NewEncoder(w).Encode(response)
}
