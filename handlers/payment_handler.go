package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"my-rinha-go/models"
	"my-rinha-go/services"

	jsoniter "github.com/json-iterator/go"
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
	json := jsoniter.ConfigFastest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	if _, err := ph.paymentService.ProcessPayment(req); err != nil {
		switch e := err.(type) {
		case *services.UpstreamStatusError:
			code := http.StatusBadGateway
			if e.StatusCode == 422 {
				code = http.StatusUnprocessableEntity
			} else if e.StatusCode == 503 || e.StatusCode == 500 {
				code = http.StatusServiceUnavailable
			}
			ph.writeErrorResponse(w, code, "Erro no processador de pagamento")
			return
		default:
			ph.writeErrorResponse(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]string{
		"status":  "success",
		"message": "Pagamento processado com sucesso",
	}
	json2 := jsoniter.ConfigFastest
	_ = json2.NewEncoder(w).Encode(response)
}

func (ph *PaymentHandler) GetPaymentsSummary(w http.ResponseWriter, r *http.Request) {
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
		ph.writeErrorResponse(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := jsoniter.ConfigFastest
	_ = enc.NewEncoder(w).Encode(summary)
}

func (ph *PaymentHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]string{
		"error": message,
	}
	_ = json.NewEncoder(w).Encode(response)
}
 