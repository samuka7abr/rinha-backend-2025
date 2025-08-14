package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"my-rinha-go/models"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type PaymentRepository struct {
	db            *sql.DB
	logger        *logrus.Logger
	enqueueCh     chan *models.PaymentRecord
	batchSize     int
	batchInterval time.Duration
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{
		db:     db,
		logger: logrus.New(),
	}
}

func (pr *PaymentRepository) SavePayment(record *models.PaymentRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	query := `
		INSERT INTO payments (correlation_id, amount, processor, processed_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := pr.db.QueryRowContext(
		ctx,
		query,
		record.CorrelationID,
		record.Amount,
		record.Processor,
		record.ProcessedAt,
		record.CreatedAt,
	).Scan(&record.ID)

	if err != nil {
		pr.logger.WithError(err).Error("Erro ao salvar pagamento no banco")
		return fmt.Errorf("erro ao salvar pagamento: %w", err)
	}
	return nil
}

func (pr *PaymentRepository) GetPaymentByCorrelationID(correlationID string) (*models.PaymentRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	query := `
		SELECT id, correlation_id, amount, processor, processed_at, created_at
		FROM payments
		WHERE correlation_id = $1
	`
	record := &models.PaymentRecord{}
	err := pr.db.QueryRowContext(ctx, query, correlationID).Scan(
		&record.ID,
		&record.CorrelationID,
		&record.Amount,
		&record.Processor,
		&record.ProcessedAt,
		&record.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		pr.logger.WithError(err).Error("Erro ao buscar pagamento no banco")
		return nil, fmt.Errorf("erro ao buscar pagamento: %w", err)
	}
	return record, nil
}

func (pr *PaymentRepository) GetPaymentsSummary(from, to *time.Time) (map[string]models.ProcessorSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	query := `
		SELECT
			processor,
			COUNT(*) as total_requests,
			SUM(amount) as total_amount
		FROM payments
		WHERE ($1::timestamp IS NULL OR processed_at >= $1)
		  AND ($2::timestamp IS NULL OR processed_at <= $2)
		GROUP BY processor
	`
	rows, err := pr.db.QueryContext(ctx, query, from, to)
	if err != nil {
		pr.logger.WithError(err).Error("Erro ao consultar resumo de pagamentos")
		return nil, fmt.Errorf("erro ao consultar resumo: %w", err)
	}
	defer rows.Close()

	summary := make(map[string]models.ProcessorSummary)
	for rows.Next() {
		var processor string
		var totalRequests int
		var totalAmount float64
		if err := rows.Scan(&processor, &totalRequests, &totalAmount); err != nil {
			pr.logger.WithError(err).Error("Erro ao fazer scan do resultado")
			return nil, fmt.Errorf("erro ao processar resultado: %w", err)
		}
		summary[processor] = models.ProcessorSummary{
			TotalRequests: totalRequests,
			TotalAmount:   totalAmount,
		}
	}
	return summary, nil
}

func (pr *PaymentRepository) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := pr.db.PingContext(ctx); err != nil {
		pr.logger.WithError(err).Error("Health check do banco falhou")
		return fmt.Errorf("banco indisponível: %w", err)
	}
	return nil
}

func (pr *PaymentRepository) StartAsyncWorkers(workers int, batchSize int, batchInterval time.Duration) {
	if pr.enqueueCh != nil {
		return
	}
	if workers <= 0 {
		workers = 2
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if batchInterval <= 0 {
		batchInterval = 3 * time.Millisecond
	}
	pr.batchSize = batchSize
	pr.batchInterval = batchInterval
	pr.enqueueCh = make(chan *models.PaymentRecord, batchSize*200)
	for i := 0; i < workers; i++ {
		go pr.worker()
	}
}

func (pr *PaymentRepository) EnqueuePayment(record *models.PaymentRecord) {
	if pr.enqueueCh == nil {
		_ = pr.SavePayment(record)
		return
	}
	select {
	case pr.enqueueCh <- record:
	default:
		// Canal cheio: pequena espera para backpressure controlado
		t := time.NewTimer(2 * time.Millisecond)
		select {
		case pr.enqueueCh <- record:
			if !t.Stop() {
				<-t.C
			}
		default:
			if !t.Stop() {
				<-t.C
			}
			pr.logger.Warn("Fila de pagamentos cheia, descartando pagamento para preservar latência")
		}
	}
}

func (pr *PaymentRepository) worker() {
	batch := make([]*models.PaymentRecord, 0, pr.batchSize)
	ticker := time.NewTicker(pr.batchInterval)
	defer ticker.Stop()
	for {
		select {
		case rec := <-pr.enqueueCh:
			if rec != nil {
				batch = append(batch, rec)
				if len(batch) >= pr.batchSize {
					pr.flushBatch(batch)
					batch = batch[:0]
				}
			}
		case <-ticker.C:
			if len(batch) > 0 {
				pr.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (pr *PaymentRepository) flushBatch(batch []*models.PaymentRecord) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	var (
		b    strings.Builder
		args []interface{}
	)
	b.WriteString("INSERT INTO payments (correlation_id, amount, processor, processed_at, created_at) VALUES ")
	for i, r := range batch {
		if i > 0 {
			b.WriteString(",")
		}
		base := i*5 + 1
		b.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", base, base+1, base+2, base+3, base+4))
		args = append(args, r.CorrelationID, r.Amount, r.Processor, r.ProcessedAt, r.CreatedAt)
	}
	_, err := pr.db.ExecContext(ctx, b.String(), args...)
	if err != nil {
		pr.logger.WithError(err).Error("Erro ao inserir batch de pagamentos")
	}
}
