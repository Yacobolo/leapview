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

	addr := listenAddr()

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

func listenAddr() string {
	if addr := os.Getenv("LIBREDASH_ADDR"); addr != "" {
		return addr
	}
	if addr := os.Getenv("ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		if port[0] == ':' {
			return port
		}
		return ":" + port
	}
	return ":8080"
}
