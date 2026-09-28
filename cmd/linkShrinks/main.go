package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"LinkShrink/internal/app"
	"LinkShrink/internal/config"
)

func main() {
	cfg, err := config.Load(os.Args[1:], os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(config.Usage)
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
