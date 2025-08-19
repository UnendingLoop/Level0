package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"orderservice/internal/cache"
	"orderservice/internal/model"
	"orderservice/internal/repository"
	"time"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"
)

type OrderService interface {
	AddNewOrder(msg *kafka.Message)
	GetOrderInfo(ctx context.Context, uid string) (*model.Order, error)
}

// OrderService provides access to repo - DB operations, and contains a Map - cached orders
type orderService struct {
	Repo repository.OrderRepository
	Map  *cache.OrderMap
}

var (
	ErrRecordNotFound = errors.New("Запрошенный номер заказа не найдет в базе!")
	ErrJSONDecode     = errors.New("Ошибка декодирования JSON-сообщения: ")
	ErrIncompleteJson = errors.New("Json содержит неполные данные")
)

// NewOrderService - returns *orderService
func NewOrderService(repo repository.OrderRepository, mapa *cache.OrderMap) OrderService {
	return &orderService{Repo: repo, Map: mapa}
}

// AddNewOrder receives rawJson from Kafka consumer and creates new order in DB if rawJSON is valid, otherwise adds broken JSON into table InvalidRequests
func (OS *orderService) AddNewOrder(msg *kafka.Message) {
	var order model.Order
	//Обработка ошибки декодирования
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		log.Printf(ErrJSONDecode.Error(), err)
		OS.pushToInvalidRequests(msg.Value, ErrJSONDecode)
		return
	}

	//Обработка ошибок валидации данных
	if !isValidOrderJSON(&order) {
		log.Println("JSON is incomplete")
		OS.pushToInvalidRequests(msg.Value, ErrJSONDecode)
		return
	}

	//Проверка на существование в кеше
	OS.Map.RLock()
	_, exists := OS.Map.CacheMap[order.OrderUID]
	OS.Map.RUnlock()
	if exists {
		log.Printf("Заказ с номером '%s' уже существует!", order.OrderUID)
		return
	}
	//Проверка на существование в БД
	if _, err := OS.GetOrderInfo(context.Background(), order.OrderUID); err == nil {
		log.Printf("Заказ с номером '%s' уже существует!", order.OrderUID)
		return
	}

	// Записываем заказ в базу
	if err := OS.Repo.AddNewOrder(context.Background(), &order); err != nil {
		log.Printf("Failed to save order %s to DB: %v", order.OrderUID, err)
		return
	}
	// Обновление кеша - можно вынести в отдельную функцию
	OS.Map.Lock()
	OS.Map.CacheMap[order.OrderUID] = order
	OS.Map.Unlock()

	log.Printf("Order '%s' created and cached", order.OrderUID)
}

// GetOrderInfo used only for API-calls, returns model.Order by its uuid from DB if there is any, or nil and error
func (OS *orderService) GetOrderInfo(ctx context.Context, uid string) (*model.Order, error) {
	//Проверяем сначала кэш
	OS.Map.RLock()
	order, ok := OS.Map.CacheMap[uid]
	OS.Map.RUnlock()

	if ok {
		return &order, nil
	}

	// В кеше нет, идем в бд:
	orderFromDB, err := OS.Repo.GetOrderByUID(ctx, uid)
	if err == nil {
		// Обновление кеша
		OS.Map.Lock()
		OS.Map.CacheMap[uid] = *orderFromDB
		OS.Map.Unlock()
		return orderFromDB, nil
	}

	//Получили ошибку из бд
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRecordNotFound
	}
	return nil, err
}

func (OS *orderService) pushToInvalidRequests(brokenJSON []byte, origErr error) {
	if err := OS.Repo.PushOrderToRawTable(context.Background(), model.InvalidRequest{
		ReceivedAt:   time.Now(),
		RawJSON:      string(brokenJSON),
		ErrorMessage: origErr.Error(),
		Status:       "New", //потом можно вынести в отдельный тип
	}); err != nil {
		log.Printf("Failed to safe order to table InvalidRequests: %v", err)
		return
	}
	log.Printf("Invalid JSON saved to InvalidRequests.")
}

func isValidOrderJSON(order *model.Order) bool {
	// Проверяем top-level поля Order
	if order.OrderUID == "" ||
		order.TrackNumber == "" ||
		order.Entry == "" ||
		order.Locale == "" ||
		order.CustomerID == "" ||
		order.DeliveryService == "" ||
		order.ShardKey == "" ||
		order.OofShard == "" ||
		order.DateCreated == "" {
		return false
	}

	// Проверяем Delivery
	d := order.Delivery
	if d.Name == "" ||
		d.Phone == "" ||
		d.Zip == "" ||
		d.City == "" ||
		d.Address == "" ||
		d.Region == "" ||
		d.Email == "" {
		return false
	}

	// Проверяем Payment
	p := order.Payment
	if p.Transaction == "" ||
		p.Currency == "" ||
		p.Provider == "" ||
		p.Amount == 0 ||
		p.PaymentDT == 0 ||
		p.Bank == "" ||
		p.GoodsTotal == 0 {
		return false
	}

	// Проверяем Items — массив не может быть пустым
	if len(order.Items) == 0 {
		return false
	}
	for _, item := range order.Items {
		if item.ChrtID == 0 ||
			item.TrackNumber == "" ||
			item.Price == 0 ||
			item.RID == "" ||
			item.Name == "" ||
			item.Size == "" ||
			item.TotalPrice == 0 ||
			item.NMID == 0 ||
			item.Brand == "" {
			return false
		}
	}

	return true
}
