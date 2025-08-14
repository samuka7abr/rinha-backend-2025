package database

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

func NewDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "admin"),
		Password: getEnv("DB_PASSWORD", "123"),
		DBName:   getEnv("DB_NAME", "rinha"),
	}
}

func (dc *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dc.Host, dc.Port, dc.User, dc.Password, dc.DBName,
	)
}

func ConnectToDatabase(config *DatabaseConfig) (*sql.DB, error) {
	logger := logrus.New()
	logger.WithFields(logrus.Fields{
		"host":   config.Host,
		"port":   config.Port,
		"dbname": config.DBName,
		"user":   config.User,
	}).Info("Tentando conectar ao banco de dados")

	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		logger.WithError(err).Error("Erro ao abrir conexão com o banco")
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err = db.Ping(); err != nil {
		db.Close()
		logger.WithError(err).Error("Erro ao fazer ping no banco")
		return nil, fmt.Errorf("erro ao testar conexão: %w", err)
	}
	logger.Info("Conexão com o banco estabelecida com sucesso")
	return db, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
 