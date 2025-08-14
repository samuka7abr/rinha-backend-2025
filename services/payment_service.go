package services

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"my-rinha-go/models"
	"my-rinha-go/repositories"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

type UpstreamStatusError struct {
	StatusCode int
	Message    string
}

func (e *UpstreamStatusError) Error() string { return e.Message }

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if ue, ok := err.(*UpstreamStatusError); ok {
		if ue.StatusCode >= 400 && ue.StatusCode < 500 {
			return false
		}
		return true
	}
	return true
}

type cbState int32

const (
	cbClosed cbState = iota
	cbOpen
	cbHalfOpen
)

type PaymentService struct {
	defaultURL   string
	fallbackURL  string
	httpClient   *http.Client
	logger       *logrus.Logger
	repository   *repositories.PaymentRepository
	redisService *RedisService

	cbFailures     atomic.Int32
	cbStatus       atomic.Int32
	cbLastOpenTime atomic.Int64
}

func NewPaymentService(repository *repositories.PaymentRepository, redisService *RedisService) *PaymentService {
	tr := &http.Transport{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 512,
		IdleConnTimeout:     60 * time.Second,
		DisableCompression:  true,
		DialContext: (&net.Dialer{
			Timeout:   200 * time.Millisecond,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   0,
		ExpectContinueTimeout: 0,
	}
	ps := &PaymentService{
		defaultURL:   getEnv("PAYMENT_PROCESSOR_DEFAULT_URL", "http://payment-processor-default:8080"),
		fallbackURL:  getEnv("PAYMENT_PROCESSOR_FALLBACK_URL", "http://payment-processor-fallback:8080"),
		httpClient:   &http.Client{Transport: tr, Timeout: 450 * time.Millisecond},
		logger:       logrus.New(),
		repository:   repository,
		redisService: redisService,
	}
	ps.cbStatus.Store(int32(cbClosed))
	return ps
}

func (ps *PaymentService) ProcessPayment(req models.PaymentRequest) (*models.PaymentRecord, error) {
	if _, err := uuid.Parse(req.CorrelationID); err != nil {
		return nil, fmt.Errorf("correlationId deve ser um UUID válido: %w", err)
	}

	if ps.redisService != nil {
		if ok, err := ps.redisService.TryLockCorrelation(req.CorrelationID, 60*time.Second); err == nil {
			if !ok {
				return &models.PaymentRecord{CorrelationID: req.CorrelationID, Amount: req.Amount, Processor: "default", ProcessedAt: time.Now().UTC(), CreatedAt: time.Now().UTC()}, nil
			}
		}
	}

	processorReq := models.PaymentProcessorRequest{
		CorrelationID: req.CorrelationID,
		Amount:        req.Amount,
		RequestedAt:   time.Now().UTC(),
	}
	bestProcessor, err := ps.getBestProcessor()
	if err != nil {
		bestProcessor = "default"
	}
	// Circuit breaker influence on routing
	s := cbState(ps.cbStatus.Load())
	if s == cbOpen {
		last := time.Unix(0, ps.cbLastOpenTime.Load())
		if time.Since(last) < 5*time.Second {
			bestProcessor = "fallback"
		} else {
			ps.cbStatus.Store(int32(cbHalfOpen))
		}
	}

	budget := 450 * time.Millisecond
	start := time.Now()
	attempt1Timeout := 320 * time.Millisecond
	attempt2Timeout := 120 * time.Millisecond

	var record *models.PaymentRecord
	var processingErr error
	if bestProcessor == "default" {
		record, processingErr = ps.tryProcessorWithMetrics(processorReq, ps.defaultURL, "default", attempt1Timeout)
		if processingErr != nil && isRetryable(processingErr) {
			if time.Since(start)+attempt2Timeout < budget {
				// jitter curto antes do retry
				time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)
				record, processingErr = ps.tryProcessorWithMetrics(processorReq, ps.fallbackURL, "fallback", attempt2Timeout)
			}
		}
	} else {
		// Quando escolhemos fallback como melhor, evitamos reverter para default para não amplificar carga
		record, processingErr = ps.tryProcessorWithMetrics(processorReq, ps.fallbackURL, "fallback", attempt1Timeout)
	}
	if processingErr != nil {
		ps.onCallResult(false)
		return nil, fmt.Errorf("falha ao processar pagamento: %w", processingErr)
	}
	ps.onCallResult(true)

	if record != nil {
		ps.repository.EnqueuePayment(record)
		if ps.redisService != nil {
			_ = ps.redisService.CachePaymentResult(req.CorrelationID, "processed")
			_ = ps.redisService.IncrementSummary(record.Processor, record.Amount)
		}
	}
	return record, nil
}

func (ps *PaymentService) onCallResult(success bool) {
	state := cbState(ps.cbStatus.Load())
	if state == cbHalfOpen {
		if success {
			ps.cbStatus.Store(int32(cbClosed))
			ps.cbFailures.Store(0)
			return
		}
		ps.cbStatus.Store(int32(cbOpen))
		ps.cbLastOpenTime.Store(time.Now().UnixNano())
		return
	}
	if !success {
		if ps.cbFailures.Add(1) >= 4 {
			ps.cbStatus.Store(int32(cbOpen))
			ps.cbLastOpenTime.Store(time.Now().UnixNano())
		}
	} else {
		ps.cbFailures.Store(0)
	}
}

func (ps *PaymentService) getBestProcessor() (string, error) {
	if ps.redisService == nil {
		return "default", nil
	}
	return ps.redisService.GetBestProcessor()
}

func (ps *PaymentService) tryProcessorWithMetrics(req models.PaymentProcessorRequest, url, processor string, timeout time.Duration) (*models.PaymentRecord, error) {
	start := time.Now()
	record, err := ps.tryProcessor(req, url, processor, timeout)
	d := time.Since(start)
	if ps.redisService != nil {
		success := err == nil
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		ps.redisService.UpdateProcessorHealth(processor, success, int(d.Milliseconds()), errMsg)
	}
	return record, err
}

func (ps *PaymentService) tryProcessor(req models.PaymentProcessorRequest, url, processor string, timeout time.Duration) (*models.PaymentRecord, error) {
	json := jsoniter.ConfigFastest
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/payments", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := ps.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnprocessableEntity { // 422: idempotente
		return &models.PaymentRecord{
			CorrelationID: req.CorrelationID,
			Amount:        req.Amount,
			Processor:     processor,
			ProcessedAt:   req.RequestedAt,
			CreatedAt:     time.Now().UTC(),
		}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("status %d", resp.StatusCode)}
	}
	record := &models.PaymentRecord{
		CorrelationID: req.CorrelationID,
		Amount:        req.Amount,
		Processor:     processor,
		ProcessedAt:   req.RequestedAt,
		CreatedAt:     time.Now().UTC(),
	}
	return record, nil
}

func (ps *PaymentService) GetPaymentsSummary(from, to *time.Time) (*models.PaymentsSummaryResponse, error) {
	if from == nil && to == nil && ps.redisService != nil {
		if live, err := ps.redisService.GetLiveSummary(); err == nil && live != nil {
			return live, nil
		}
	}
	summaryKey := ps.generateSummaryKey(from, to)
	if ps.redisService != nil {
		if cached, err := ps.redisService.GetCachedPaymentsSummary(summaryKey); err == nil && cached != nil {
			return cached, nil
		}
	}
	summaryMap, err := ps.repository.GetPaymentsSummary(from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao obter summary: %w", err)
	}
	response := &models.PaymentsSummaryResponse{}
	if s, ok := summaryMap["default"]; ok {
		response.Default = s
	}
	if s, ok := summaryMap["fallback"]; ok {
		response.Fallback = s
	}
	if ps.redisService != nil {
		_ = ps.redisService.CachePaymentsSummary(summaryKey, response)
	}
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
