package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type ProcessorHealth struct {
	Healthy          bool      `json:"healthy"`
	LastResponseTime int       `json:"lastResponseTime"` // em ms
	LastCheck        time.Time `json:"lastCheck"`
	SuccessRate      float64   `json:"successRate"`
	FailureCount     int       `json:"failureCount"`
	TotalRequests    int       `json:"totalRequests"`
	LastError        string    `json:"lastError,omitempty"`
}

type RedisService struct {
	client *redis.Client
	logger *logrus.Logger
	ctx    context.Context
}

func NewRedisService() *RedisService {
	addr := getEnv("REDIS_ADDR", "localhost:6379")

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "",
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
	})

	rs := &RedisService{
		client: client,
		logger: logrus.New(),
		ctx:    context.Background(),
	}

	if err := rs.HealthCheck(); err != nil {
		rs.logger.WithError(err).Warn("Redis não disponível, funcionando sem cache")
	} else {
		rs.logger.Info("Redis conectado com sucesso")
	}

	return rs
}

func (rs *RedisService) HealthCheck() error {
	ctx, cancel := context.WithTimeout(rs.ctx, 2*time.Second)
	defer cancel()

	return rs.client.Ping(ctx).Err()
}

func (rs *RedisService) GetProcessorHealth(processor string) (*ProcessorHealth, error) {
	ctx, cancel := context.WithTimeout(rs.ctx, 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("health:%s", processor)

	data, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao buscar health do Redis: %w", err)
	}

	var health ProcessorHealth
	if err := json.Unmarshal([]byte(data), &health); err != nil {
		return nil, fmt.Errorf("erro ao deserializar health: %w", err)
	}

	return &health, nil
}

func (rs *RedisService) SetProcessorHealth(processor string, health *ProcessorHealth) error {
	ctx, cancel := context.WithTimeout(rs.ctx, 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("health:%s", processor)

	data, err := json.Marshal(health)
	if err != nil {
		return fmt.Errorf("erro ao serializar health: %w", err)
	}

	if err := rs.client.Set(ctx, key, data, 5*time.Second).Err(); err != nil {
		return fmt.Errorf("erro ao salvar health no Redis: %w", err)
	}

	rs.logger.WithFields(logrus.Fields{
		"processor":    processor,
		"healthy":      health.Healthy,
		"responseTime": health.LastResponseTime,
		"successRate":  health.SuccessRate,
	}).Debug("Health salvo no Redis")

	return nil
}

func (rs *RedisService) UpdateProcessorHealth(processor string, success bool, responseTime int, errorMsg string) {
	health, err := rs.GetProcessorHealth(processor)
	if err != nil {
		rs.logger.WithError(err).Warn("Erro ao obter health do Redis")
		health = &ProcessorHealth{
			Healthy:       true,
			SuccessRate:   1.0,
			TotalRequests: 0,
			FailureCount:  0,
		}
	}

	if health == nil {
		health = &ProcessorHealth{
			Healthy:       true,
			SuccessRate:   1.0,
			TotalRequests: 0,
			FailureCount:  0,
		}
	}

	health.TotalRequests++
	health.LastCheck = time.Now()
	health.LastResponseTime = responseTime

	if success {
		health.Healthy = true
		health.LastError = ""
	} else {
		health.FailureCount++
		health.Healthy = false
		health.LastError = errorMsg
	}

	if health.TotalRequests > 0 {
		health.SuccessRate = float64(health.TotalRequests-health.FailureCount) / float64(health.TotalRequests)
	}

	if health.FailureCount >= 3 {
		health.Healthy = false
	}

	if health.SuccessRate < 0.8 {
		health.Healthy = false
	}

	if err := rs.SetProcessorHealth(processor, health); err != nil {
		rs.logger.WithError(err).Warn("Erro ao salvar health no Redis")
	}
}

func (rs *RedisService) GetBestProcessor() (string, error) {
	defaultHealth, err := rs.GetProcessorHealth("default")
	if err != nil {
		rs.logger.WithError(err).Warn("Erro ao obter health do default")
	}

	fallbackHealth, err := rs.GetProcessorHealth("fallback")
	if err != nil {
		rs.logger.WithError(err).Warn("Erro ao obter health do fallback")
	}

	if defaultHealth == nil && fallbackHealth == nil {
		return "default", nil
	}

	if defaultHealth == nil && fallbackHealth != nil {
		if fallbackHealth.Healthy {
			return "fallback", nil
		}
		return "default", nil
	}

	if fallbackHealth == nil && defaultHealth != nil {
		if defaultHealth.Healthy {
			return "default", nil
		}
		return "fallback", nil
	}

	rs.logger.WithFields(logrus.Fields{
		"default_healthy":        defaultHealth.Healthy,
		"default_response_time":  defaultHealth.LastResponseTime,
		"default_success_rate":   defaultHealth.SuccessRate,
		"fallback_healthy":       fallbackHealth.Healthy,
		"fallback_response_time": fallbackHealth.LastResponseTime,
		"fallback_success_rate":  fallbackHealth.SuccessRate,
	}).Debug("Comparando processadores")

	if defaultHealth.Healthy && !fallbackHealth.Healthy {
		return "default", nil
	}

	if !defaultHealth.Healthy && fallbackHealth.Healthy {
		return "fallback", nil
	}

	if defaultHealth.Healthy && fallbackHealth.Healthy {
		fallbackThreshold := float64(fallbackHealth.LastResponseTime) * 1.5
		if float64(defaultHealth.LastResponseTime) <= fallbackThreshold {
			return "default", nil
		}
		return "fallback", nil
	}

	return "default", nil
}

func (rs *RedisService) CachePaymentResult(correlationID string, result string) error {
	ctx, cancel := context.WithTimeout(rs.ctx, 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("payment:%s", correlationID)

	if err := rs.client.Set(ctx, key, result, 1*time.Hour).Err(); err != nil {
		return fmt.Errorf("erro ao cachear resultado: %w", err)
	}

	return nil
}

func (rs *RedisService) GetCachedPaymentResult(correlationID string) (string, error) {
	ctx, cancel := context.WithTimeout(rs.ctx, 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("payment:%s", correlationID)

	result, err := rs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("erro ao buscar cache: %w", err)
	}

	return result, nil
}
