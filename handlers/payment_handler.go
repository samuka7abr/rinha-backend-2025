package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"myRinhaGo/models"
	"myRinhaGo/services"
)

type PaymentHandler struct {
	Svc services.PaymentService
}

func NewPaymentHandler(svc services.PaymentService) *PaymentHandler { return &PaymentHandler{Svc: svc} }

func (h *PaymentHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("POST /payments", h.handlePayments)
	mux.HandleFunc("GET /payments-summary", h.handleSummary)
	mux.HandleFunc("POST /purge-payments", h.handlePurge)
}

func (h *PaymentHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

func (h *PaymentHandler) handlePayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if r.Body == nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	body, err := io.ReadAll(io.LimitReader(r.Body, 256))
	if err != nil || len(body) == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var p models.Payment
	if err := json.Unmarshal(body, &p); err != nil || p.Amount == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	h.Svc.Add(r.Context(), p)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *PaymentHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var from, to time.Time
	var err error
	if q.Get("from") != "" && q.Get("to") != "" {
		from, err = time.Parse(time.RFC3339, q.Get("from"))
		if err != nil {
			http.Error(w, "bad from", 400); return
		}
		to, err = time.Parse(time.RFC3339, q.Get("to"))
		if err != nil {
			http.Error(w, "bad to", 400); return
		}
	} else {
		to = time.Now()
		from = to.Add(-60 * time.Second)
	}
	sum, cnt := h.Svc.Summary(from.Unix(), to.Unix())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sum": sum, "count": cnt})
}

func (h *PaymentHandler) handlePurge(w http.ResponseWriter, r *http.Request) {
	h.Svc.Purge(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"purged":true}`))
}
