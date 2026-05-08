package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	kafkaContracts "github.com/jva44ka/marketplace-simulator-product/api_internal/kafka"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ProductEventRecordBuilder struct {
	oldStates map[uint64]models.Product
}

func NewProductEventRecordBuilder(oldStates map[uint64]models.Product) *ProductEventRecordBuilder {
	return &ProductEventRecordBuilder{
		oldStates: oldStates,
	}
}

func (s *ProductEventRecordBuilder) BuildRecords(
	ctx context.Context, newStates map[uint64]models.Product) (
	[]models.ProductEventOutboxRecordNew, error) {
	if len(s.oldStates) != len(newStates) {
		return nil, fmt.Errorf("oldStates and newStates are not the same length")
	}

	records := make([]models.ProductEventOutboxRecordNew, 0, len(newStates))

	for sku, newState := range newStates {
		oldState, ok := s.oldStates[sku]
		if !ok {
			return nil, fmt.Errorf("old state not found for sku: %d", sku)
		}

		data, err := json.Marshal(kafkaContracts.ProductEventData{
			Old: toSnapshot(oldState),
			New: toSnapshot(newState),
		})
		if err != nil {
			return nil, fmt.Errorf("OutboxService.SaveProductChanged: marshal: %w", err)
		}

		headers := make(map[string]string)
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))

		records = append(records, models.ProductEventOutboxRecordNew{
			Key:     strconv.FormatUint(newState.Sku, 10),
			Data:    data,
			Headers: headers,
		})
	}

	return records, nil
}

func toSnapshot(p models.Product) kafkaContracts.Product {
	return kafkaContracts.Product{
		Sku:   p.Sku,
		Name:  p.Name,
		Price: p.Price,
		Count: p.Count - p.ReservedCount,
	}
}
