package cache

import (
	"context"
	"log"
	"orderservice/internal/model"
	"orderservice/internal/repository"
	"sync"
)

// OrderMap provides access to cache-map, contains embedded mutex features
type OrderMap struct {
	CacheMap     map[string]model.Order
	Repo         repository.OrderRepository
	sync.RWMutex //встраиваем методы мютекса для защиты
}

// CreateAndWarmUpOrderCache returns a new map with warmed up cache, access to DB and embedded mutex
func CreateAndWarmUpOrderCache(repo repository.OrderRepository) (*OrderMap, error) {
	simpleMap := make(map[string]model.Order)
	orderMap := OrderMap{Repo: repo}
	orders, err := orderMap.Repo.GetAllOrders(context.Background())
	if err != nil {
		log.Printf("Failed to read orders from DB to warm up cahce: %v", err)
		return nil, err
	}

	for _, v := range orders {
		simpleMap[v.OrderUID] = v
	}
	orderMap.CacheMap = simpleMap
	log.Println("Cache successfully loaded!")
	return &orderMap, nil
}
