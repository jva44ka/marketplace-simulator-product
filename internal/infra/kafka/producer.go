package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	segkafka "github.com/segmentio/kafka-go"
)

type ReservationExpiredEvent struct {
	ReservationId int64  `json:"reservation_id"`
	Sku           uint64 `json:"sku"`
	Count         uint32 `json:"count"`
}

type Producer struct {
	writer *segkafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &segkafka.Writer{
			Addr:  segkafka.TCP(brokers...),
			Topic: topic,
		},
	}
}

func (p *Producer) PublishReservationExpired(ctx context.Context, id int64, sku uint64, count uint32) error {
	event := ReservationExpiredEvent{
		ReservationId: id,
		Sku:           sku,
		Count:         count,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Producer.PublishReservationExpired: marshal: %w", err)
	}

	return p.writer.WriteMessages(ctx, segkafka.Message{
		Key:   []byte(strconv.FormatInt(id, 10)),
		Value: data,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
