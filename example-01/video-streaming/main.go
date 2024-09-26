package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
)

const (
	contentLength = "Content-Length"
	contentType   = "Content-Type"
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
	mux.HandleFunc("GET /video", func(w http.ResponseWriter, r *http.Request) {
		videoPath := "./videos/SampleVideo_1280x720_1mb.mp4"
		videoReader, err := os.Open(videoPath)
		if err != nil {
			log.Error("Not Found")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer videoReader.Close()
		videoStats, err := videoReader.Stat()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add(contentLength, strconv.FormatInt(videoStats.Size(), 10))
		w.Header().Add(contentType, "video/mp4")
		// use io.Copy for streaming.
		io.Copy(w, videoReader)
	})

	log.Info(`Microservice online`)
	return http.ListenAndServe(fmt.Sprint(":", port), mux)

}
