// Package model - provides all data-structs for the main application
package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CustomTime used for converting time fields from Kafka-JSON into time.Time
// пока не использую, так как gorm передает CustomTime в базу в виде текста, а не времени - пока не нашел решение
type CustomTime struct {
	time.Time
}

// Order is a complete model with embedded structs for storing order information received from Kafka
type Order struct {
	OrderUID    string `gorm:"primaryKey" json:"order_uid" faker:"-"`
	TrackNumber string `gorm:"not null" json:"track_number" faker:"word"`
	Entry       string `gorm:"not null" json:"entry" faker:"word"`

	Delivery Delivery `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:OrderUID;references:OrderUID" json:"delivery"`
	Payment  Payment  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:OrderUID;references:OrderUID" json:"payment"`
	Items    []Item   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:OrderUID;references:OrderUID" json:"items" faker:"slice_len=2"`

	Locale            string `gorm:"not null" json:"locale" faker:"word"`
	InternalSignature string `gorm:"not null" json:"internal_signature" faker:"word"`
	CustomerID        string `gorm:"not null" json:"customer_id" faker:"word"`
	DeliveryService   string `gorm:"not null" json:"delivery_service" faker:"word"`
	ShardKey          string `gorm:"not null" json:"shardkey" faker:"word"`
	SMID              int    `gorm:"not null" json:"sm_id" faker:"number"`
	DateCreated       string `gorm:"not null" json:"date_created" faker:"date"`
	OofShard          string `gorm:"not null" json:"oof_shard" faker:"word"`
}

// Delivery contains delivery information for a certain order
type Delivery struct {
	DID      *uint  `gorm:"primaryKey;autoIncrement;->" json:"-" faker:"-"`
	OrderUID string `gorm:"index;not null" faker:"-"` // FK на Order.OrderUID
	Name     string `gorm:"not null" json:"name" faker:"name"`
	Phone    string `gorm:"not null" json:"phone" faker:"-"`
	Zip      string `gorm:"not null" json:"zip" faker:"word"`
	City     string `gorm:"not null" json:"city" faker:"word"`
	Address  string `gorm:"not null" json:"address" faker:"word"`
	Region   string `gorm:"not null" json:"region"  faker:"word"`
	Email    string `gorm:"not null" json:"email" faker:"email"`
}

// Payment contains payment information for a certain order
type Payment struct {
	PID          *uint  `gorm:"primaryKey;autoIncrement;->" json:"-" faker:"-"`
	OrderUID     string `gorm:"index;not null;index" faker:"-"` // FK на Order.OrderUID
	Transaction  string `gorm:"not null" json:"transaction" faker:"word"`
	RequestID    string `gorm:"not null" json:"request_id" faker:"word"`
	Currency     string `gorm:"not null" json:"currency" faker:"word"`
	Provider     string `gorm:"not null" json:"provider" faker:"word"`
	Amount       uint   `gorm:"not null" json:"amount" faker:"number"`     // должно быть суммой DeliveryCost+GoodsTotal+CustomFee
	PaymentDT    uint   `gorm:"not null" json:"payment_dt" faker:"number"` // unix время в секундах
	Bank         string `gorm:"not null" json:"bank" faker:"word"`
	DeliveryCost uint   `gorm:"not null" json:"delivery_cost" faker:"number"`
	GoodsTotal   uint   `gorm:"not null" json:"goods_total"`
	CustomFee    uint   `gorm:"not null" json:"custom_fee" faker:"number"`
}

// Item is a struct for items in an order, presented as an array in model.Order, cannot be empty(!)
type Item struct {
	IID         *uint  `gorm:"primaryKey;autoIncrement;->" json:"-" faker:"-"`
	OrderUID    string `gorm:"index;not null;index" faker:"-"` // FK на Order.OrderUID
	ChrtID      uint   `gorm:"not null" json:"chrt_id" faker:"number"`
	TrackNumber string `gorm:"not null" json:"track_number" faker:"-"`
	Price       uint   `gorm:"not null" json:"price" faker:"number"`
	RID         string `gorm:"not null" json:"rid" faker:"word"`
	Name        string `gorm:"not null" json:"name" faker:"word"`
	Sale        uint   `gorm:"not null" json:"sale" faker:"-"` // ожидается процент скидки 0-100%
	Size        string `gorm:"not null" json:"size" faker:"word"`
	TotalPrice  uint   `gorm:"not null" json:"total_price" faker:"-"` // будем считать, что итоговая цена товара округляется в меньшую сторону до рублей после применения скидки
	NMID        uint   `gorm:"not null" json:"nm_id" faker:"number"`
	Brand       string `gorm:"not null" json:"brand" faker:"word"`
	Status      int    `gorm:"not null" json:"status" faker:"number"`
}

// UnmarshalJSON - method for CustomTime used to process "RFC3339" and "Unix timestamp" input date types
func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// пробуем как RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		ct.Time = t.UTC()
		return nil
	}

	// пробуем как Unix timestamp
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		ct.Time = time.Unix(ts, 0).UTC()
		return nil
	}

	return fmt.Errorf("неизвестный формат времени: %s", s)
}
