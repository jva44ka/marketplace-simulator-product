package models

type Product struct {
	Sku           uint64
	Price         float64
	Name          string
	Count         uint32
	TransactionId uint32
}
