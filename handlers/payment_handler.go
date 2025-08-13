package handlers

import (
	"encoding/json"
	"net/http"
	"time"

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

func (ph *PaymentHandler) GetPaymentsSummary(w http.ResponseWriter, r *http.Request) {
	// Verifica se é GET
	if r.Method != http.MethodGet {
		ph.writeErrorResponse(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}

	var from, to *time.Time

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if parsedFrom, err := time.Parse(time.RFC3339, fromStr); err != nil {
			ph.writeErrorResponse(w, http.StatusBadRequest, "Parâmetro 'from' inválido (use formato RFC3339)")
			return
		} else {
			from = &parsedFrom
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if parsedTo, err := time.Parse(time.RFC3339, toStr); err != nil {
			ph.writeErrorResponse(w, http.StatusBadRequest, "Parâmetro 'to' inválido (use formato RFC3339)")
			return
		} else {
			to = &parsedTo
		}
	}

	summary, err := ph.paymentService.GetPaymentsSummary(from, to)
	if err != nil {
		ph.logger.WithError(err).Error("Erro ao obter summary de pagamentos")
		ph.writeErrorResponse(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}

	ph.logger.WithFields(logrus.Fields{
		"from":             from,
		"to":               to,
		"defaultRequests":  summary.Default.TotalRequests,
		"defaultAmount":    summary.Default.TotalAmount,
		"fallbackRequests": summary.Fallback.TotalRequests,
		"fallbackAmount":   summary.Fallback.TotalAmount,
	}).Info("Summary consultado com sucesso")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(summary); err != nil {
		ph.logger.WithError(err).Error("Erro ao serializar resposta JSON")
		return
	}
}

func (ph *PaymentHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error": message,
	}
	json.NewEncoder(w).Encode(response)
}
