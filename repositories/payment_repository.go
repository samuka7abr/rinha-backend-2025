package repositories

import (
	"database/sql"
	"fmt"
	"time"

	"my-rinha-go/models"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type PaymentRepository struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{
		db:     db,
		logger: logrus.New(),
	}
}

func (pr *PaymentRepository) SavePayment(record *models.PaymentRecord) error {
	query := `
		INSERT INTO payments (correlation_id, amount, processor, processed_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	err := pr.db.QueryRow(
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

	pr.logger.WithFields(logrus.Fields{
		"id":            record.ID,
		"correlationId": record.CorrelationID,
		"processor":     record.Processor,
		"amount":        record.Amount,
	}).Info("Pagamento salvo no banco com sucesso")

	return nil
}

func (pr *PaymentRepository) GetPaymentByCorrelationID(correlationID string) (*models.PaymentRecord, error) {
	query := `
		SELECT id, correlation_id, amount, processor, processed_at, created_at
		FROM payments
		WHERE correlation_id = $1
	`

	record := &models.PaymentRecord{}
	err := pr.db.QueryRow(query, correlationID).Scan(
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

	rows, err := pr.db.Query(query, from, to)
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

		err := rows.Scan(&processor, &totalRequests, &totalAmount)
		if err != nil {
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
	err := pr.db.Ping()
	if err != nil {
		pr.logger.WithError(err).Error("Health check do banco falhou")
		return fmt.Errorf("banco indisponível: %w", err)
	}
	return nil
}
