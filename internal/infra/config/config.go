package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HttpServer struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"http-server"`

	GrpcServer struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"grpc-server"`

	Database struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Name     string `yaml:"name"`
	} `yaml:"database"`

	Authorization struct {
		Enabled   bool   `yaml:"enabled"`
		AdminUser string `yaml:"admin-user"`
	} `yaml:"authorization"`

	Logging struct {
		LogRequestBody  bool `yaml:"log-request-body"`
		LogResponseBody bool `yaml:"log-response-body"`
	} `yaml:"logging"`

	Kafka struct {
		Brokers            []string `yaml:"brokers"`
		ProductEventsTopic string   `yaml:"product-events-topic"`
		WriteTimeout       string   `yaml:"write-timeout"`
	} `yaml:"kafka"`

	Jobs struct {
		ReservationExpiry          ReservationExpiryConfig          `yaml:"reservation-expiry"`
		ProductEventsOutbox        ProductEventsOutboxConfig        `yaml:"product-events-outbox"`
		ProductEventsOutboxMonitor ProductEventsOutboxMonitorConfig `yaml:"product-events-outbox-monitor"`
	} `yaml:"jobs"`
}

func LoadConfig(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	config := &Config{}
	if err = yaml.NewDecoder(f).Decode(config); err != nil {
		return nil, err
	}

	return config, nil
}

type ReservationExpiryConfig struct {
	Enabled     bool   `yaml:"enabled"`
	TTL         string `yaml:"ttl"`
	JobInterval string `yaml:"job-interval"`
}

type ProductEventsOutboxConfig struct {
	Enabled     bool   `yaml:"enabled"`
	JobInterval string `yaml:"job-interval"`
	BatchSize   int    `yaml:"batch-size"`
	MaxRetries  int    `yaml:"max-retries"`
}

type ProductEventsOutboxMonitorConfig struct {
	Enabled     bool   `yaml:"enabled"`
	JobInterval string `yaml:"job-interval"`
}
