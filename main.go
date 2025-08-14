package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"my-rinha-go/database"
	"my-rinha-go/handlers"
	"my-rinha-go/repositories"
	"my-rinha-go/services"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Olá, Rinha de Backend!")
}

func main() {
	runtime.GOMAXPROCS(2)
	dbConfig := database.NewDatabaseConfig()
	db, err := database.ConnectToDatabase(dbConfig)
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco de dados: %v", err)
	}
	defer db.Close()

	paymentRepository := repositories.NewPaymentRepository(db)

	if err := paymentRepository.HealthCheck(); err != nil {
		log.Fatalf("Health check do banco falhou: %v", err)
	}

	paymentRepository.StartAsyncWorkers(4, 150, 3*time.Millisecond)

	redisService := services.NewRedisService()
	paymentService := services.NewPaymentService(paymentRepository, redisService)
	paymentHandler := handlers.NewPaymentHandler(paymentService)

	ctx := context.Background()
	hu := services.NewHealthUpdater(redisService)
	hu.Start(ctx)

	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/payments", paymentHandler.PostPayments)
	http.HandleFunc("/payments-summary", paymentHandler.GetPaymentsSummary)

	port := "8080"
	fmt.Printf("Servidor iniciado na porta %s\n", port)
	fmt.Println("Endpoints disponíveis:")
	fmt.Println("  GET  / - Página inicial")
	fmt.Println("  POST /payments - Processar pagamentos (com estratégia inteligente)")
	fmt.Println("  GET  /payments-summary - Resumo de pagamentos (com cache)")
	fmt.Println("Banco de dados:", "PostgreSQL conectado")
	fmt.Println("Cache:", "Redis conectado")
	fmt.Println("Estratégia:", "Roteamento inteligente baseado em métricas")

	s := &http.Server{
		Addr:              ":" + port,
		Handler:           nil,
		ReadHeaderTimeout: 750 * time.Millisecond,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	log.Fatal(s.ListenAndServe())
}
