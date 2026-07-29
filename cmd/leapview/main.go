package main

import (
	"context"
	"log"

	"github.com/flidai/leapview/internal/app/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		log.Fatal(err)
	}
}
