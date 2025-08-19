package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CustomTime used for converting time fields from Kafka-JSON into time.Time
type CustomTime struct {
	time.Time
}

// Order is a complete model with embedded structs for storing order information received from Kafka
type Order struct {
	OrderUID    string `gorm:"primaryKey" json:"order_uid"`
	TrackNumber string `gorm:"not null" json:"track_number"`
	Entry       string `gorm:"not null" json:"entry"`

	Delivery Delivery `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:OrderUID;references:OrderUID" json:"delivery"`
	Payment  Payment  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:OrderUID;references:OrderUID" json:"payment"`
	Items    []Item   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:OrderUID;references:OrderUID" json:"items"`

	Locale            string `gorm:"not null" json:"locale"`
	InternalSignature string `gorm:"not null" json:"internal_signature"`
	CustomerID        string `gorm:"not null" json:"customer_id"`
	DeliveryService   string `gorm:"not null" json:"delivery_service"`
	ShardKey          string `gorm:"not null" json:"shardkey"`
	SMID              int    `gorm:"not null" json:"sm_id"`
	DateCreated       string `gorm:"not null" json:"date_created"`
	OofShard          string `gorm:"not null" json:"oof_shard"`
}

// Delivery contains delivery information for a certain order
type Delivery struct {
	DID      *uint  `gorm:"primaryKey;autoIncrement;->" json:"-"`
	OrderUID string `gorm:"index;not null"` // FK на Order.OrderUID
	Name     string `gorm:"not null" json:"name"`
	Phone    string `gorm:"not null" json:"phone"`
	Zip      string `gorm:"not null" json:"zip"`
	City     string `gorm:"not null" json:"city"`
	Address  string `gorm:"not null" json:"address"`
	Region   string `gorm:"not null" json:"region"`
	Email    string `gorm:"not null" json:"email"`
}

// Payment contains payment information for a certain order
type Payment struct {
	PID          *uint  `gorm:"primaryKey;autoIncrement;->" json:"-"`
	OrderUID     string `gorm:"index;not null;index"` // FK на Order.OrderUID
	Transaction  string `gorm:"not null" json:"transaction"`
	RequestID    string `gorm:"not null" json:"request_id"`
	Currency     string `gorm:"not null" json:"currency"`
	Provider     string `gorm:"not null" json:"provider"`
	Amount       uint   `gorm:"not null" json:"amount"`
	PaymentDT    uint   `gorm:"not null" json:"payment_dt"`
	Bank         string `gorm:"not null" json:"bank"`
	DeliveryCost uint   `gorm:"not null" json:"delivery_cost"`
	GoodsTotal   uint   `gorm:"not null" json:"goods_total"`
	CustomFee    uint   `gorm:"not null" json:"custom_fee"`
}

// Item is a struct for item in an order, presented as an array in model.Order, cannot be empty(!)
type Item struct {
	IID         *uint  `gorm:"primaryKey;autoIncrement;->" json:"-"`
	OrderUID    string `gorm:"index;not null;index"` // FK на Order.OrderUID
	ChrtID      uint   `gorm:"not null" json:"chrt_id"`
	TrackNumber string `gorm:"not null" json:"track_number"`
	Price       uint   `gorm:"not null" json:"price"`
	RID         string `gorm:"not null" json:"rid"`
	Name        string `gorm:"not null" json:"name"`
	Sale        uint   `gorm:"not null" json:"sale"`
	Size        string `gorm:"not null" json:"size"`
	TotalPrice  uint   `gorm:"not null" json:"total_price"`
	NMID        uint   `gorm:"not null" json:"nm_id"`
	Brand       string `gorm:"not null" json:"brand"`
	Status      int    `gorm:"not null" json:"status"`
}

// InvalidRequest is a struct for storing order information if it is received from Kafka in invalid form
type InvalidRequest struct {
	ID           *uint     `gorm:"primaryKey;autoIncrement;->" json:"-"`
	ReceivedAt   time.Time `gorm:"not null" json:"-"`
	RawJSON      string    `gorm:"not null" json:"-"`
	ErrorMessage string    `gorm:"not null" json:"-"`
	Status       string    `gorm:"not null" json:"-"` //may be used for assigning statuses like "new","processed"
}

// UnmarshalJSON - method for CustomTime used to process "RFC3339" and "Unix timestamp" input date types
// не забыть добавить сохранение заказа в RAW-табличку в слое сервиса
func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}

	//пробуем как RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		ct.Time = t.UTC()
		return nil
	}

	//пробуем как Unix timestamp
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		ct.Time = time.Unix(ts, 0).UTC()
		return nil
	}

	return fmt.Errorf("неизвестный формат времени: %s", s)
}
