package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"myRinhaGo/database"
	"myRinhaGo/handlers"
	"myRinhaGo/repositories"
	"myRinhaGo/services"
)

func main() {
	log.SetFlags(0)
	port := getenv("APP_PORT", "8080")
	window := getint("WINDOW_SECONDS", 600)
	queueSize := getint("QUEUE_SIZE", 200000)
	batchSize := getint("BATCH_SIZE", 2000)
	batchMax := time.Duration(getint("BATCH_MAX_MS", 50)) * time.Millisecond

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := database.NewPool(ctx)
	if err != nil {
		log.Fatalf("pg: %v", err)
	}
	defer pool.Close()

	writer := repositories.NewPaymentWriter(pool, queueSize, batchSize, batchMax)
	go writer.Run(ctx)

	rds, err := services.NewRedisService(ctx)
	if err != nil {
		log.Printf("redis degraded: %v", err)
		rds = nil
	}

	services.StartHealthUpdater(ctx, pool)

	svc := services.NewPaymentService(int64(window), writer, rds)
	h := handlers.NewPaymentHandler(svc)

	mux := http.NewServeMux()
	h.Register(mux)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       2 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	_ = srv.Shutdown(ctx2)
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func getint(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return d
}
