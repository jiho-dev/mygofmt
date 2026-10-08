package main

import (
	"github.com/jiho-dev/mygofmt/internal/config"
	"github.com/jiho-dev/mygofmt/internal/formatter"
)

func Format(source []byte, cfg config.Config) ([]byte, error) {
	return formatter.Format(source, cfg)
}
