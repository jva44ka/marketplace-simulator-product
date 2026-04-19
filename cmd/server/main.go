//go:generate swag init -g cmd/server/main.go --dir ./internal,./cmd
package main

import (
	"context"
	"log/slog"
	"os"

	appPkg "github.com/jva44ka/marketplace-simulator-product/internal/app"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
)

func main() {
	configImpl, err := config.LoadConfig(os.Getenv("CONFIG_PATH"))
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	app, err := appPkg.NewApp(ctx, configImpl)
	if err != nil {
		slog.Error("failed to create app", "err", err)
		os.Exit(1)
	}

	if err = app.Run(ctx); err != nil {
		slog.Error("failed to run app", "err", err)
		os.Exit(1)
	}
}
