package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"my-rinha-go/handlers"
	"my-rinha-go/models"

	"github.com/google/uuid"
)

type MockPaymentService struct {
	shouldError        bool
	processor          string
	summaryShouldError bool
	summaryResp        *models.PaymentsSummaryResponse
}

func (m *MockPaymentService) ProcessPayment(req models.PaymentRequest) (*models.PaymentRecord, error) {
	if m.shouldError {
		return nil, fmt.Errorf("erro simulado do processador")
	}

	processor := "default"
	if m.processor != "" {
		processor = m.processor
	}

	return &models.PaymentRecord{
		ID:            1,
		CorrelationID: req.CorrelationID,
		Amount:        req.Amount,
		Processor:     processor,
		ProcessedAt:   time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (m *MockPaymentService) GetPaymentsSummary(from, to *time.Time) (*models.PaymentsSummaryResponse, error) {
	if m.summaryShouldError {
		return nil, fmt.Errorf("erro simulado no summary")
	}
	if m.summaryResp != nil {
		return m.summaryResp, nil
	}
	return &models.PaymentsSummaryResponse{
		Default:  models.ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		Fallback: models.ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
	}, nil
}

func TestPaymentHandler_PostPayments(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
		mockError      bool
	}{
		{
			name: "Requisição válida",
			requestBody: models.PaymentRequest{
				CorrelationID: uuid.New().String(),
				Amount:        100.50,
			},
			expectedStatus: http.StatusOK,
			mockError:      false,
		},
		{
			name:           "JSON inválido",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "JSON inválido",
		},
		{
			name: "CorrelationID vazio",
			requestBody: models.PaymentRequest{
				CorrelationID: "",
				Amount:        100.50,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "correlationId é obrigatório",
		},
		{
			name: "Amount zero",
			requestBody: models.PaymentRequest{
				CorrelationID: uuid.New().String(),
				Amount:        0,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "amount deve ser maior que 0",
		},
		{
			name: "Amount negativo",
			requestBody: models.PaymentRequest{
				CorrelationID: uuid.New().String(),
				Amount:        -10.50,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "amount deve ser maior que 0",
		},
		{
			name: "Erro no processamento",
			requestBody: models.PaymentRequest{
				CorrelationID: uuid.New().String(),
				Amount:        100.50,
			},
			expectedStatus: http.StatusInternalServerError,
			mockError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPaymentService{shouldError: tt.mockError}
			handler := handlers.NewPaymentHandler(mockService)

			var bodyBytes []byte
			if str, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, _ = json.Marshal(tt.requestBody)
			}

			req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler.PostPayments(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Esperado status %d, obtido %d", tt.expectedStatus, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Esperado Content-Type application/json, obtido %s", contentType)
			}

			if tt.expectedError != "" {
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Erro ao decodificar resposta JSON: %v", err)
				}

				if response["error"] != tt.expectedError {
					t.Errorf("Esperado erro '%s', obtido '%s'", tt.expectedError, response["error"])
				}
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Erro ao decodificar resposta JSON: %v", err)
				}

				if response["status"] != "success" {
					t.Errorf("Esperado status 'success', obtido '%s'", response["status"])
				}

				if response["message"] == "" {
					t.Error("Esperado message não vazio")
				}
			}
		})
	}
}

func TestPaymentHandler_PostPayments_MethodNotAllowed(t *testing.T) {
	mockService := &MockPaymentService{}
	handler := handlers.NewPaymentHandler(mockService)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run("Method_"+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/payments", nil)
			w := httptest.NewRecorder()

			handler.PostPayments(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Esperado status %d para método %s, obtido %d",
					http.StatusMethodNotAllowed, method, w.Code)
			}

			var response map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Erro ao decodificar resposta JSON: %v", err)
			}

			if response["error"] != "Método não permitido" {
				t.Errorf("Esperado erro 'Método não permitido', obtido '%s'", response["error"])
			}
		})
	}
}

func TestPaymentHandler_PostPayments_EmptyBody(t *testing.T) {
	mockService := &MockPaymentService{}
	handler := handlers.NewPaymentHandler(mockService)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostPayments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Esperado status %d, obtido %d", http.StatusBadRequest, w.Code)
	}
}

func TestPaymentHandler_GetPaymentsSummary_OK(t *testing.T) {
	summary := &models.PaymentsSummaryResponse{
		Default:  models.ProcessorSummary{TotalRequests: 10, TotalAmount: 1000},
		Fallback: models.ProcessorSummary{TotalRequests: 2, TotalAmount: 200},
	}
	mockService := &MockPaymentService{summaryResp: summary}
	handler := handlers.NewPaymentHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/payments-summary", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentsSummary(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Esperado status %d, obtido %d", http.StatusOK, w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Esperado Content-Type application/json, obtido %s", ct)
	}

	var resp models.PaymentsSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}

	if resp.Default.TotalRequests != 10 || resp.Default.TotalAmount != 1000 {
		t.Errorf("Resumo default incorreto: %+v", resp.Default)
	}
	if resp.Fallback.TotalRequests != 2 || resp.Fallback.TotalAmount != 200 {
		t.Errorf("Resumo fallback incorreto: %+v", resp.Fallback)
	}
}

func TestPaymentHandler_GetPaymentsSummary_WithFilters(t *testing.T) {
	summary := &models.PaymentsSummaryResponse{
		Default:  models.ProcessorSummary{TotalRequests: 1, TotalAmount: 50},
		Fallback: models.ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
	}
	mockService := &MockPaymentService{summaryResp: summary}
	handler := handlers.NewPaymentHandler(mockService)

	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/payments-summary?from="+from+"&to="+to, nil)
	w := httptest.NewRecorder()

	handler.GetPaymentsSummary(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Esperado status %d, obtido %d", http.StatusOK, w.Code)
	}
}

func TestPaymentHandler_GetPaymentsSummary_InvalidFrom(t *testing.T) {
	mockService := &MockPaymentService{}
	handler := handlers.NewPaymentHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/payments-summary?from=invalid-date", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentsSummary(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Esperado status %d, obtido %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}
	if response["error"] != "Parâmetro 'from' inválido (use formato RFC3339)" {
		t.Errorf("Mensagem de erro incorreta: %s", response["error"])
	}
}

func TestPaymentHandler_GetPaymentsSummary_InvalidTo(t *testing.T) {
	mockService := &MockPaymentService{}
	handler := handlers.NewPaymentHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/payments-summary?to=invalid-date", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentsSummary(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Esperado status %d, obtido %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}
	if response["error"] != "Parâmetro 'to' inválido (use formato RFC3339)" {
		t.Errorf("Mensagem de erro incorreta: %s", response["error"])
	}
}

func TestPaymentHandler_GetPaymentsSummary_ServiceError(t *testing.T) {
	mockService := &MockPaymentService{summaryShouldError: true}
	handler := handlers.NewPaymentHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/payments-summary", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentsSummary(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Esperado status %d, obtido %d", http.StatusInternalServerError, w.Code)
	}
}

func TestPaymentHandler_GetPaymentsSummary_MethodNotAllowed(t *testing.T) {
	mockService := &MockPaymentService{}
	handler := handlers.NewPaymentHandler(mockService)

	req := httptest.NewRequest(http.MethodPost, "/payments-summary", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentsSummary(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Esperado status %d, obtido %d", http.StatusMethodNotAllowed, w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}
	if response["error"] != "Método não permitido" {
		t.Errorf("Mensagem de erro incorreta: %s", response["error"])
	}
}
