package etcd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"

	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
)

// NewClient создаёт подключение к etcd.
func NewClient(cfg *config.EtcdConfig) (*clientv3.Client, error) {
	dialTimeout, err := time.ParseDuration(cfg.DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse etcd dial-timeout: %w", err)
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd.New: %w", err)
	}

	return client, nil
}

// ReadFromEtcd читает конфиг из etcd. Возвращает (cfg, found, err).
// Конфиг хранится в формате YAML — том же, что и файл конфигурации.
func ReadFromEtcd(ctx context.Context, client *clientv3.Client, key string) (*config.Config, bool, error) {
	resp, err := client.Get(ctx, key)
	if err != nil {
		return nil, false, fmt.Errorf("etcd.Get(%q): %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, false, nil
	}

	var cfg config.Config
	if err := yaml.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
		return nil, true, fmt.Errorf("etcd: unmarshal config from %q: %w", key, err)
	}

	return &cfg, true, nil
}

// SeedIfAbsent записывает cfg в etcd под ключом key только если ключ ещё не существует.
// Используется при первом старте для инициализации etcd из yaml-конфига.
func SeedIfAbsent(ctx context.Context, client *clientv3.Client, key string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("etcd: marshal config: %w", err)
	}

	// Conditional put: записываем только если ключ не существует (version == 0)
	txnResp, err := client.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(key), "=", 0)).
		Then(clientv3.OpPut(key, string(data))).
		Commit()
	if err != nil {
		return fmt.Errorf("etcd: seed txn for %q: %w", key, err)
	}

	if txnResp.Succeeded {
		slog.Info("etcd: seeded config from yaml", "key", key)
	} else {
		slog.Info("etcd: config key already exists, skipping seed", "key", key)
	}

	return nil
}
