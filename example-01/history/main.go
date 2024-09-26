package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error(`[fatal]`, `err`, err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	port, found := os.LookupEnv(`PORT`)
	if !found {
		fmt.Errorf(`Please specify the port number for the HTTP server with the environment variable PORT.`)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
	})

	log.Info(`Microservice online`)
	return http.ListenAndServe(fmt.Sprint(":", port), mux)
}
