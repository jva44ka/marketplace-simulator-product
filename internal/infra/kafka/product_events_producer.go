package kafka

import (
	"context"
	"time"

	kafkaContracts "github.com/jva44ka/ozon-simulator-go-products/api_internal/kafka"
)

type ProductEventsProducer struct {
	//TODO: заменить на интерфейс
	producer *Producer[uint64, kafkaContracts.ProductEventBody]
}

func NewProductEventsProducer(brokers []string, topic string, writeTimeout time.Duration) *ProductEventsProducer {
	return &ProductEventsProducer{
		producer: NewProducer[uint64, kafkaContracts.ProductEventBody](brokers, topic, writeTimeout),
	}
}

func (p *ProductEventsProducer) PublishProductChangedBatch(ctx context.Context, events []kafkaContracts.ProductEventMessage) error {
	messages := make([]Message[uint64, kafkaContracts.ProductEventBody], 0, len(events))

	for _, event := range events {
		messages = append(messages, Message[uint64, kafkaContracts.ProductEventBody]{
			Key:     event.Key,
			Value:   event.Body,
			Headers: event.Headers,
		})
	}

	return p.producer.WriteBatch(ctx, messages)
}

func (p *ProductEventsProducer) Close() error {
	return p.producer.Close()
}
