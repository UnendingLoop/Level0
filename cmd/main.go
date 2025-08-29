package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"orderservice/config"
	handler "orderservice/internal/api"
	"orderservice/internal/cache"
	"orderservice/internal/db"
	"orderservice/internal/kafka"
	"orderservice/internal/repository"
	"orderservice/internal/service"
	"orderservice/internal/web"

	"github.com/go-chi/chi/v5"
)

func main() {
	// читаем конфиг из env
	startConfig := config.GetConfig()
	fmt.Println("Start config values:", startConfig)

	// подключаемся к базе
	db := db.ConnectPostgres(startConfig.DSN)
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to retrieve sql.DB: %v", err)
	}
	defer sqlDB.Close()

	// создаем экземпляр repository и прогреваем кэш
	repo := repository.NewOrderRepository(db, startConfig.DSN)
	orderMap, err := cache.CreateAndWarmUpOrderCache(repo, startConfig.CacheSize)
	if err != nil {
		log.Fatalf("Failed to load cache: %v", err)
	}

	// создаем экземпляры слоя сервиса и хэндлера
	svc := service.NewOrderService(repo, orderMap, startConfig.KafkaBroker, startConfig.DLQTopic)
	hndlr := handler.OrderHandler{
		Service: svc,
	}

	// настраиваем роутер и грузим настройки сервера
	r := chi.NewRouter()
	r.Get("/order/{uid}", hndlr.GetOrderInfo)
	r.Get("/order/", hndlr.GetOrderInfo)
	srv := config.LoadSrvConfig(r, startConfig.AppPort)

	wg := sync.WaitGroup{}

	// запускаем сервер в отдельной горутине, чтобы можно было:
	// - вызвать Shutdown сервера из слушателя прерываний
	// - выполнить последующий код
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Printf("Server running on http://localhost:%s\n", startConfig.AppPort)
		err := srv.ListenAndServe()
		if err != nil {
			switch {
			case errors.Is(err, http.ErrServerClosed):
				log.Println("Server gracefully stopping...")
			default:
				log.Fatalf("Server stopped: %v", err)
			}
		}
	}()

	// грузим шаблоны страниц для веба
	web.LoadTemplates()

	// ждем пока кафка запустится
	kafka.WaitKafkaReady(startConfig.KafkaBroker)

	// Cоздаем топики
	kafka.InitKafkaTopics(startConfig.KafkaBroker, startConfig.Topic, startConfig.DLQTopic)

	// запускаем консюмер для чтения из кафки
	ctx, kafkaCancel := context.WithCancel(context.Background())
	wg.Add(1)
	go kafka.StartConsumer(ctx, hndlr.Service, startConfig.KafkaBroker, startConfig.Topic, &wg)
	time.Sleep(3 * time.Second)

	// запуск мокового писателя в кафку для теста
	if startConfig.LaunchMockGenerator {
		go kafka.EmulateMsgSending(startConfig.KafkaBroker, startConfig.Topic)
	}

	// Starting shutdown signal listener
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM) // слушатель сигналов Ctrl+C и SIGTERM - когда убиваем контейнеры
	defer close(sig)

	wg.Add(1)
	go func() {
		log.Print("Interruption listener is running...")
		defer wg.Done()
		<-sig
		log.Println("Interrupt received!!! Starting shutdown sequence...")
		// stop Kafka consumer:
		kafkaCancel()
		log.Println("Kafka consumer stopping...")
		// 5 seconds to stop HTTP-server:
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
