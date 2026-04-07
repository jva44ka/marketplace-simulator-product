package product_events_outbox

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
)

type ProductSnapshot struct {
	Sku   uint64  `json:"sku"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Count uint32  `json:"count"`
}

// TODO: вынести контракт
type ProductEventData struct {
	Old ProductSnapshot `json:"old"`
	New ProductSnapshot `json:"new"`
}

type RecordBuilder struct {
	oldStates map[uint64]models.Product
}

func NewRecordBuilder(oldStates map[uint64]models.Product) *RecordBuilder {
	return &RecordBuilder{
		oldStates: oldStates,
	}
}

func (s *RecordBuilder) BuildRecords(
	newStates map[uint64]models.Product) ([]models.ProductEventOutboxRecordNew, error) {
	if len(s.oldStates) != len(newStates) {
		return nil, fmt.Errorf("oldStates and newStates are not the same length")
	}

	records := make([]models.ProductEventOutboxRecordNew, 0, len(newStates))

	for sku, newState := range newStates {
		oldState, ok := s.oldStates[sku]
		if !ok {
			return nil, fmt.Errorf("old state not found for sku: %d", sku)
		}

		data, err := json.Marshal(ProductEventData{
			Old: toSnapshot(oldState),
			New: toSnapshot(newState),
		})
		if err != nil {
			return nil, fmt.Errorf("OutboxService.SaveProductChanged: marshal: %w", err)
		}

		records = append(records, models.ProductEventOutboxRecordNew{
			Key:  strconv.FormatUint(newState.Sku, 10),
			Data: string(data),
		})
	}

	return records, nil
}

func toSnapshot(p models.Product) ProductSnapshot {
	return ProductSnapshot{
		Sku:   p.Sku,
		Name:  p.Name,
		Price: p.Price,
		Count: p.Count,
	}
}
