package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
)

const (
	contentLength = "Content-Length"
	contentType   = "Content-Type"
)

type viewedMessageBody struct {
	VideoPath string `json:"videoPath" bson:"videoPath"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error(`[fatal]`, `err`, err)
		os.Exit(1)
	}
}

func sendViewedMessage(videoPath string) {
	// Create the request body
	viewedMessageBody := viewedMessageBody{
		VideoPath: videoPath,
	}

	// Create a new POST request
	req, _ := http.NewRequest(http.MethodPost, "http://history/viewed", nil)

	// Set the content type header
	req.Header.Set("Content-Type", "application/json")

	// Create a buffer to hold the JSON data
	var jsonBuffer bytes.Buffer

	// Use json.NewEncoder to encode the request body directly into the request
	json.NewEncoder(&jsonBuffer).Encode(viewedMessageBody)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Failed to send 'viewed' message!")
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	// Log the response status
	log.Println("Sent 'viewed' message to history microservice.")
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
		io.Copy(w, videoReader)
		sendViewedMessage(videoPath)
	})

	log.Info(`Microservice online`)
	return http.ListenAndServe(":"+port, mux)
}
