package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"my-rinha-go/models"
	"my-rinha-go/repositories"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type PaymentService struct {
	defaultURL   string
	fallbackURL  string
	httpClient   *http.Client
	logger       *logrus.Logger
	repository   *repositories.PaymentRepository
	redisService *RedisService
}

func NewPaymentService(repository *repositories.PaymentRepository, redisService *RedisService) *PaymentService {
	return &PaymentService{
		defaultURL:  getEnv("PAYMENT_PROCESSOR_DEFAULT_URL", "http://payment-processor-default:8080"),
		fallbackURL: getEnv("PAYMENT_PROCESSOR_FALLBACK_URL", "http://payment-processor-fallback:8080"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:       logrus.New(),
		repository:   repository,
		redisService: redisService,
	}
}

func (ps *PaymentService) ProcessPayment(req models.PaymentRequest) (*models.PaymentRecord, error) {
	if _, err := uuid.Parse(req.CorrelationID); err != nil {
		return nil, fmt.Errorf("correlationId deve ser um UUID válido: %w", err)
	}

	if ps.redisService != nil {
		cached, err := ps.redisService.GetCachedPaymentResult(req.CorrelationID)
		if err != nil {
			ps.logger.WithError(err).Warn("Erro ao verificar cache Redis")
		} else if cached != "" {
			ps.logger.WithField("correlationId", req.CorrelationID).Info("Pagamento encontrado no cache Redis")
			if record, err := ps.repository.GetPaymentByCorrelationID(req.CorrelationID); err == nil && record != nil {
				return record, nil
			}
		}
	}

	existingPayment, err := ps.repository.GetPaymentByCorrelationID(req.CorrelationID)
	if err != nil {
		ps.logger.WithError(err).Warn("Erro ao verificar pagamento existente no banco")
	} else if existingPayment != nil {
		ps.logger.WithField("correlationId", req.CorrelationID).Info("Pagamento duplicado detectado no banco")

		if ps.redisService != nil {
			ps.redisService.CachePaymentResult(req.CorrelationID, "found")
		}

		return existingPayment, nil
	}

	processorReq := models.PaymentProcessorRequest{
		CorrelationID: req.CorrelationID,
		Amount:        req.Amount,
		RequestedAt:   time.Now().UTC(),
	}

	bestProcessor, err := ps.getBestProcessor()
	if err != nil {
		ps.logger.WithError(err).Warn("Erro ao determinar melhor processador, usando default")
		bestProcessor = "default"
	}

	ps.logger.WithField("chosenProcessor", bestProcessor).Info("Processador escolhido pela estratégia inteligente")

	var record *models.PaymentRecord
	var processingErr error

	if bestProcessor == "default" {
		record, processingErr = ps.tryProcessorWithMetrics(processorReq, ps.defaultURL, "default")
		if processingErr != nil {
			ps.logger.WithError(processingErr).Warn("Falha no processador default, tentando fallback")
			record, processingErr = ps.tryProcessorWithMetrics(processorReq, ps.fallbackURL, "fallback")
		}
	} else {
		record, processingErr = ps.tryProcessorWithMetrics(processorReq, ps.fallbackURL, "fallback")
		if processingErr != nil {
			ps.logger.WithError(processingErr).Warn("Falha no processador fallback, tentando default")
			record, processingErr = ps.tryProcessorWithMetrics(processorReq, ps.defaultURL, "default")
		}
	}

	if processingErr != nil {
		ps.logger.WithError(processingErr).Error("Falha em ambos os processadores")
		return nil, fmt.Errorf("falha ao processar pagamento: %w", processingErr)
	}

	if err := ps.repository.SavePayment(record); err != nil {
		ps.logger.WithError(err).Error("Erro ao salvar pagamento no banco")
	}

	if ps.redisService != nil {
		if err := ps.redisService.CachePaymentResult(req.CorrelationID, "processed"); err != nil {
			ps.logger.WithError(err).Warn("Erro ao cachear resultado no Redis")
		}
	}

	ps.logger.WithFields(logrus.Fields{
		"correlationId": req.CorrelationID,
		"amount":        req.Amount,
		"processor":     record.Processor,
	}).Info("Pagamento processado com sucesso")

	return record, nil
}

func (ps *PaymentService) getBestProcessor() (string, error) {
	if ps.redisService == nil {
		return "default", nil
	}

	return ps.redisService.GetBestProcessor()
}

func (ps *PaymentService) tryProcessorWithMetrics(req models.PaymentProcessorRequest, url, processor string) (*models.PaymentRecord, error) {
	start := time.Now()

	record, err := ps.tryProcessor(req, url, processor)

	duration := time.Since(start)
	responseTimeMs := int(duration.Milliseconds())

	if ps.redisService != nil {
		success := err == nil
		errorMsg := ""
		if err != nil {
			errorMsg = err.Error()
		}

		ps.redisService.UpdateProcessorHealth(processor, success, responseTimeMs, errorMsg)
	}

	return record, err
}

func (ps *PaymentService) tryProcessor(req models.PaymentProcessorRequest, url, processor string) (*models.PaymentRecord, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar requisição: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url+"/payments", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição HTTP: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := ps.httpClient.Do(httpReq)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("erro na requisição HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("processador retornou status %d", resp.StatusCode)
	}

	record := &models.PaymentRecord{
		CorrelationID: req.CorrelationID,
		Amount:        req.Amount,
		Processor:     processor,
		ProcessedAt:   req.RequestedAt,
		CreatedAt:     time.Now().UTC(),
	}

	ps.logger.WithFields(logrus.Fields{
		"processor": processor,
		"duration":  duration,
		"status":    resp.StatusCode,
	}).Debug("Requisição para processador concluída")

	return record, nil
}

func (ps *PaymentService) GetPaymentsSummary(from, to *time.Time) (*models.PaymentsSummaryResponse, error) {
	summaryKey := ps.generateSummaryKey(from, to)

	if ps.redisService != nil {
		cached, err := ps.redisService.GetCachedPaymentsSummary(summaryKey)
		if err != nil {
			ps.logger.WithError(err).Warn("Erro ao verificar cache do summary")
		} else if cached != nil {
			ps.logger.WithField("summaryKey", summaryKey).Debug("Summary encontrado no cache Redis")
			return cached, nil
		}
	}

	ps.logger.WithFields(logrus.Fields{
		"from": from,
		"to":   to,
	}).Info("Consultando summary no banco de dados")

	summaryMap, err := ps.repository.GetPaymentsSummary(from, to)
	if err != nil {
		ps.logger.WithError(err).Error("Erro ao consultar summary no banco")
		return nil, fmt.Errorf("erro ao obter summary: %w", err)
	}

	response := &models.PaymentsSummaryResponse{}

	if defaultSummary, exists := summaryMap["default"]; exists {
		response.Default = defaultSummary
	} else {
		response.Default = models.ProcessorSummary{
			TotalRequests: 0,
			TotalAmount:   0,
		}
	}

	if fallbackSummary, exists := summaryMap["fallback"]; exists {
		response.Fallback = fallbackSummary
	} else {
		response.Fallback = models.ProcessorSummary{
			TotalRequests: 0,
			TotalAmount:   0,
		}
	}

	if ps.redisService != nil {
		if err := ps.redisService.CachePaymentsSummary(summaryKey, response); err != nil {
			ps.logger.WithError(err).Warn("Erro ao cachear summary no Redis")
		}
	}

	ps.logger.WithFields(logrus.Fields{
		"defaultRequests":  response.Default.TotalRequests,
		"defaultAmount":    response.Default.TotalAmount,
		"fallbackRequests": response.Fallback.TotalRequests,
		"fallbackAmount":   response.Fallback.TotalAmount,
	}).Info("Summary consultado com sucesso")

	return response, nil
}

func (ps *PaymentService) generateSummaryKey(from, to *time.Time) string {
	key := "summary"

	if from != nil {
		key += fmt.Sprintf(":from:%d", from.Unix())
	}

	if to != nil {
		key += fmt.Sprintf(":to:%d", to.Unix())
	}

	if from == nil && to == nil {
		key += ":all"
	}

	return key
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
