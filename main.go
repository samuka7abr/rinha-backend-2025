package main

import (
	"fmt"
	"log"
	"net/http"

	"my-rinha-go/database"
	"my-rinha-go/handlers"
	"my-rinha-go/repositories"
	"my-rinha-go/services"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Olá, Rinha de Backend!")
}

func main() {
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

	redisService := services.NewRedisService()

	paymentService := services.NewPaymentService(paymentRepository, redisService)

	paymentHandler := handlers.NewPaymentHandler(paymentService)

	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/payments", paymentHandler.PostPayments)

	port := "8080"
	fmt.Printf("Servidor iniciado na porta %s\n", port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
