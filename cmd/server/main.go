//go:generate swag init -g cmd/server/main.go --dir ./internal,./cmd
package main

import (
	"log"
	"os"

	appPkg "github.com/jva44ka/ozon-simulator-go-products/internal/app"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/config"
)

func main() {
	configImpl, err := config.LoadConfig(os.Getenv("CONFIG_PATH"))
	if err != nil {
		log.Fatal("config.LoadConfig: %w", err)
	}

	app, err := appPkg.NewApp(configImpl)
	if err != nil {
		log.Fatal(err)
	}

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
