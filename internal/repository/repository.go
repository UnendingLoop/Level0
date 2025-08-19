package repository

import (
	"context"
	"fmt"
	"log"
	"orderservice/internal/model"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type OrderRepository interface {
	AddNewOrder(ctx context.Context, neworder *model.Order) error
	GetOrderByUID(ctx context.Context, uid string) (*model.Order, error)
	PushOrderToRawTable(ctx context.Context, brokenOrder model.InvalidRequest) error
	GetAllOrders(ctx context.Context, count int) ([]model.Order, error)
}

type orderRepository struct {
	DB           *gorm.DB
	dsn          string      //для переподключения если отвалилась база
	reconnecting atomic.Bool //флаг запущенного переподключения к БД
	sync.Mutex               //для предотвращения множественного вызова connectWithRetry из других экземпляров хендлеров при отвале БД
}

func NewOrderRepository(db *gorm.DB, dsnDB string) OrderRepository {
	return &orderRepository{DB: db, dsn: dsnDB}
}

// GetOrderByUID finds order by its UUID and provides it with error message(if any)
func (OR *orderRepository) GetOrderByUID(ctx context.Context, uid string) (*model.Order, error) {
	var order model.Order
	for range 3 { //ограничимся тройным циклом вместо рекурсивного вызова всей AddNewOrder
		err := OR.DB.WithContext(ctx).Preload("Delivery").Preload("Payment").Preload("Items").Where("order_uid = ?", uid).First(&order).Error
		if err == nil { //если успешно - сразу выходим из цикла и функции
			return &order, nil
		}

		if isConnectionError(err) {
			switch OR.reconnecting.Load() {
			case true: //если ошибка соединения и уже запущено переподключение - ждем и пробуем снова
				time.Sleep(15 * time.Second)
				continue
			case false:
				if conErr := OR.connectWithRetry(); conErr != nil { //если не получилось восстановить соединение с одной попытки - выход из функции
					return nil, conErr
				}
				continue
			}
		}
		return nil, err
	}
	return &order, nil
}

// AddNewOrder creates a new record in DB using ctx and transaction
func (OR *orderRepository) AddNewOrder(ctx context.Context, neworder *model.Order) error {
	var tx *gorm.DB
	neworder.Delivery.DID = nil
	neworder.Payment.PID = nil
	for i := range neworder.Items {
		neworder.Items[i].IID = nil
	}

	auxFunc := func() error {
		//вспомогательная функция со всеми транзакциями
		tx = OR.DB.WithContext(ctx).Begin()
		if err := tx.Create(&neworder).Error; err != nil {
			tx.Rollback()
			return err
		}
		neworder.Delivery.OrderUID = neworder.OrderUID
		if err := tx.Create(&neworder.Delivery).Error; err != nil {
			tx.Rollback()
			return err
		}
		neworder.Payment.OrderUID = neworder.OrderUID
		if err := tx.Create(&neworder.Payment).Error; err != nil {
			tx.Rollback()
			return err
		}
		for i := range neworder.Items {
			neworder.Items[i].OrderUID = neworder.OrderUID
		}
		if err := tx.Create(&neworder.Items).Error; err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			return err
		}
		return nil
	}
	for range 3 { //ограничимся тройным циклом вместо рекурсивного вызова всей AddNewOrder
		err := auxFunc()
		if err == nil { //если успешно - сразу выходим из цикла и функции
			return nil
		}

		if isConnectionError(err) {
			switch OR.reconnecting.Load() {
			case true: //если ошибка соединения и уже запущено переподключение - ждем и пробуем снова
				time.Sleep(15 * time.Second)
				continue
			case false:
				if conErr := OR.connectWithRetry(); conErr != nil { //если не получилось восстановить соединение с одной попытки - выход из функции
					return conErr
				}
				continue
			}
		}
		return err
	}
	return nil
}

// GetAllOrders retreives existing orders from DB with limit=count, used for warming up cache at app launch
func (OR *orderRepository) GetAllOrders(ctx context.Context, count int) ([]model.Order, error) {
	var orders []model.Order

	for range 3 { //ограничимся тройным циклом вместо рекурсивного вызова всей GetAllOrders
		err := OR.DB.WithContext(ctx).Preload("Delivery").Preload("Payment").Preload("Items").Order("date_created DESC").Limit(count).Find(&orders).Error
		if err == nil { //если успешно - сразу выходим из цикла и функции
			return orders, nil
		}

		if isConnectionError(err) {
			switch OR.reconnecting.Load() {
			case true: //если ошибка соединения и уже запущено переподключение - ждем и пробуем снова
				time.Sleep(15 * time.Second)
				continue
			case false:
				if conErr := OR.connectWithRetry(); conErr != nil { //если не получилось восстановить соединение с одной попытки - выход из функции
					return nil, conErr
				}
				continue
			}
		}
		return nil, err
	}
	return orders, nil
}

// PushOrderToRawTable adds invalid JSONs into separate table for further investigation
func (OR *orderRepository) PushOrderToRawTable(ctx context.Context, brokenOrder model.InvalidRequest) error {
	brokenOrder.ID = nil
	for range 3 { //ограничимся тройным циклом вместо рекурсивного вызова всей PushOrderToRawTable
		err := OR.DB.Create(&brokenOrder).Error
		if err == nil { //если успешно - сразу выходим из цикла и функции
			return nil
		}
		if isConnectionError(err) {
			switch OR.reconnecting.Load() {
			case true: //если ошибка соединения и уже запущено переподключение - ждем и пробуем снова
				time.Sleep(15 * time.Second)
				continue
			case false:
				if conErr := OR.connectWithRetry(); conErr != nil { //если не получилось восстановить соединение с одной попытки - выход из функции
					return conErr
				}
				continue
			}
		}
		return err
	}
	return nil
}

func (OR *orderRepository) connectWithRetry() error {
	OR.reconnecting.Store(true)
	defer OR.reconnecting.Store(false)

	OR.Lock()
	defer OR.Unlock()
	var db *gorm.DB
	var err error
	maxRetries := 3
	delay := 3 * time.Second

	if sqlDB, err := OR.DB.DB(); err == nil {
		if errPing := sqlDB.Ping(); errPing == nil {
			return nil // соединение уже живое
		}
	}

	for i := 0; i < maxRetries; i++ {
		log.Printf("#%d attempt reconnecting to DB...", i+1)
		db, err = gorm.Open(postgres.Open(OR.dsn), &gorm.Config{})
		if err == nil {
			sqlDB, _ := db.DB()
			if pingErr := sqlDB.Ping(); pingErr == nil {
				OR.DB = db
				log.Println("Successfully reconnected!")
				return nil
			} else {
				err = pingErr
			}
		}
		time.Sleep(delay)
	}

	return fmt.Errorf("Could not reconnect after %d retries: %w", maxRetries, err)
}

func isConnectionError(err error) bool {
	return strings.Contains(err.Error(), "bad connection") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset")
}
