package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"orderservice/config"
	handler "orderservice/internal/api"
	"orderservice/internal/cache"
	"orderservice/internal/db"
	"orderservice/internal/kafka"
	"orderservice/internal/repository"
	"orderservice/internal/service"
	"orderservice/internal/web"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	config := config.GetConfig()
	fmt.Println(config)
	db := db.ConnectPostgres(config.DSN)

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to retrieve sql.DB: %v", err)
	}
	defer sqlDB.Close()

	repo := repository.NewOrderRepository(db, config.DSN)
	orderMap, err := cache.CreateAndWarmUpOrderCache(repo)
	if err != nil {
		log.Fatalf("Failed to load cache: %v", err)
	}
	svc := service.NewOrderService(repo, orderMap)
	orderHandler := handler.OrderHandler{
		Service: svc,
	}

	r := chi.NewRouter()
	r.Get("/order/{uid}", orderHandler.GetOrderInfo)
	r.Get("/order/", orderHandler.GetOrderInfo)
	srv := http.Server{
		Addr:         (":" + config.AppPort),
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Printf("Server running on http://localhost:%s\n", config.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server stopped: %v", err)
		}
		log.Println("Server gracefully stopping...")
	}()

	web.LoadTemplates()

	kafka.WaitKafkaReady(config.KafkaBroker)

	ctx, kafkaCancel := context.WithCancel(context.Background())
	wg.Add(1)
	go kafka.StartConsumer(ctx, orderHandler.Service, config.KafkaBroker, config.Topic, &wg)
	time.Sleep(3 * time.Second)

	if config.LaunchMockGenerator {
		go kafka.EmulateMsgSending(config.KafkaBroker, config.Topic)
	}

	//Starting shutdown signal listener
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer close(sig)

	wg.Add(1)
	go func() {
		log.Print("Interruption listener is running...")
		defer wg.Done()
		<-sig
		log.Println("Interrupt received!!! Starting shutdown sequence...")
		//stop Kafka consumer:
		kafkaCancel()
		log.Println("Kafka consumer stopping...")
		//5 seconds to stop HTTP-server:
		ctx, httpCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer httpCancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		log.Println("HTTP server stopped")
	}()
	wg.Wait()
	log.Println("Exiting application...")
}
