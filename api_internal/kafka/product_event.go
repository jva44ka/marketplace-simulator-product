package kafka

import (
	"github.com/google/uuid"
)

type ProductEventMessage struct {
	Key     uint64
	Headers map[string]string
	Body    ProductEventBody
}

type ProductEventBody struct {
	RecordId uuid.UUID        `json:"recordId"`
	Data     ProductEventData `json:"data"`
}

type ProductEventData struct {
	Old Product `json:"old"`
	New Product `json:"new"`
}

type Product struct {
	Sku   uint64  `json:"sku"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Count uint32  `json:"count"`
}
