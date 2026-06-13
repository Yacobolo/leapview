package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Yacobolo/libredash/internal/app"
	"github.com/Yacobolo/libredash/internal/data"
)

func main() {
	dataDir := os.Getenv("LIBREDASH_DATA_DIR")
	if dataDir == "" {
		dataDir = ".data/olist"
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	metrics, err := data.NewDuckDBMetrics(dataDir)
	if err != nil {
		log.Fatalf("initializing DuckDB metrics: %v", err)
	}
	defer metrics.Close()

	server := app.New(metrics)

	log.Printf("LibreDash listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
