package redis

type Product struct {
	Sku           uint64  `json:"sku"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Count         uint32  `json:"count"`
	ReservedCount uint32  `json:"reserved_count"`
	TransactionId uint32  `json:"transaction_id"`
}
