package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	segkafka "github.com/segmentio/kafka-go"
)

type Key interface {
	~uint64 | ~int64
}

type Message[K Key, V any] struct {
	Key     K
	Value   V
	Headers map[string]string
}

type Producer[K Key, V any] struct {
	writer *segkafka.Writer
}

func NewProducer[K Key, V any](brokers []string, topic string, writeTimeout time.Duration) *Producer[K, V] {
	return &Producer[K, V]{
		writer: &segkafka.Writer{
			Addr:         segkafka.TCP(brokers...),
			Topic:        topic,
			WriteTimeout: writeTimeout,
		},
	}
}

func (p *Producer[K, V]) WriteBatch(ctx context.Context, messages []Message[K, V]) error {
	segkafkaMessages := make([]segkafka.Message, 0, len(messages))
	for _, msg := range messages {
		key, err := toKeyBytes(msg.Key)
		if err != nil {
			return fmt.Errorf("Producer.WriteBatch: unknown key type=%v: %w", msg.Key, err)
		}

		headers := make([]segkafka.Header, 0, len(msg.Headers))
		for k, v := range msg.Headers {
			headers = append(headers, segkafka.Header{Key: k, Value: []byte(v)})
		}

		value, err := json.Marshal(msg.Value)
		if err != nil {
			return fmt.Errorf("Producer.WriteBatch: marshal key=%v: %w", msg.Key, err)
		}

		segkafkaMessages = append(segkafkaMessages, segkafka.Message{
			Key:     key,
			Value:   value,
			Headers: headers,
		})
	}

	return p.writer.WriteMessages(ctx, segkafkaMessages...)
}

func toKeyBytes[K Key](key K) ([]byte, error) {
	switch v := any(key).(type) {
	case uint64:
		return []byte(strconv.FormatUint(v, 10)), nil
	case int64:
		return []byte(strconv.FormatInt(v, 10)), nil
	}

	return nil, fmt.Errorf("toKeyBytes: unknown type %T", key)
}

func (p *Producer[K, V]) Close() error {
	return p.writer.Close()
}
