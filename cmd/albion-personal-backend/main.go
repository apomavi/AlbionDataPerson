package main

import (
	"github.com/ao-data/albiondata-client/backend"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := backend.ParseConfig()
	service := backend.NewService(cfg)
	if err := service.Run(); err != nil {
		logrus.Fatal(err)
	}
}
