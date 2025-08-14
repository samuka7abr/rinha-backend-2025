package services

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

type healthPayload struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

type HealthUpdater struct {
	defaultURL   string
	fallbackURL  string
	client       *http.Client
	redisService *RedisService
}

func NewHealthUpdater(redisService *RedisService) *HealthUpdater {
	return &HealthUpdater{
		defaultURL:   getEnvHU("PAYMENT_PROCESSOR_DEFAULT_URL", "http://payment-processor-default:8080") + "/payments/service-health",
		fallbackURL:  getEnvHU("PAYMENT_PROCESSOR_FALLBACK_URL", "http://payment-processor-fallback:8080") + "/payments/service-health",
		client:       &http.Client{Timeout: 500 * time.Millisecond},
		redisService: redisService,
	}
}

func (hu *HealthUpdater) Start(ctx context.Context) {
	if hu.redisService == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hu.pollOnce()
			}
		}
	}()
}

func (hu *HealthUpdater) pollOnce() {
	// default
	if p, ok := hu.fetch(hu.defaultURL); ok {
		hu.redisService.SetProcessorHealth("default", &ProcessorHealth{
			Healthy:          !p.Failing,
			LastResponseTime: p.MinResponseTime,
			LastCheck:        time.Now(),
			SuccessRate:      1.0,
		})
	}
	// fallback
	if p, ok := hu.fetch(hu.fallbackURL); ok {
		hu.redisService.SetProcessorHealth("fallback", &ProcessorHealth{
			Healthy:          !p.Failing,
			LastResponseTime: p.MinResponseTime,
			LastCheck:        time.Now(),
			SuccessRate:      1.0,
		})
	}
}

func (hu *HealthUpdater) fetch(url string) (*healthPayload, bool) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	resp, err := hu.client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	var p healthPayload
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, false
	}
	return &p, true
}

func getEnvHU(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
