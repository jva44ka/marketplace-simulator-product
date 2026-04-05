package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	segkafka "github.com/segmentio/kafka-go"
)

type ProductSnapshot struct {
	Sku   uint64  `json:"sku"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Count uint32  `json:"count"`
}

type ProductChangedEvent struct {
	Old ProductSnapshot `json:"old"`
	New ProductSnapshot `json:"new"`
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

func (p *Producer) PublishProductChanged(ctx context.Context, old, new ProductSnapshot) error {
	event := ProductChangedEvent{Old: old, New: new}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Producer.PublishProductChanged: marshal: %w", err)
	}

	return p.writer.WriteMessages(ctx, segkafka.Message{
		Key:   []byte(strconv.FormatUint(new.Sku, 10)),
		Value: data,
	})
}

func (p *Producer) PublishProductChangedBatch(ctx context.Context, events []ProductChangedEvent) error {
	msgs := make([]segkafka.Message, 0, len(events))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("Producer.PublishProductChangedBatch: marshal sku=%d: %w", event.New.Sku, err)
		}
		msgs = append(msgs, segkafka.Message{
			Key:   []byte(strconv.FormatUint(event.New.Sku, 10)),
			Value: data,
		})
	}
	return p.writer.WriteMessages(ctx, msgs...)
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
