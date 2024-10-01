package main

import (
	"bytes"
	"encoding/json"
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

type viewedMessageBody struct {
	VideoPath string `json:"videoPath"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error(`run failed`, `err`, err.Error())
	}
}

func run(log *slog.Logger) error {
	port, found := os.LookupEnv(`PORT`)
	if !found {
		return fmt.Errorf(`Please specify the port number for the HTTP server with the environment variable PORT.`)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(`GET /video`, func(w http.ResponseWriter, r *http.Request) {
		videoPath := `./videos/SampleVideo_1280x720_1mb.mp4`
		videoReader, err := os.Open(videoPath)
		if err != nil {
			log.Error(`/video.os.Open`, `err`, err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer videoReader.Close()
		videoStats, err := videoReader.Stat()
		if err != nil {
			log.Error(`/videoReader.Stat`, `err`, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add(contentLength, strconv.FormatInt(videoStats.Size(), 10))
		w.Header().Add(contentType, `video/mp4`)
		// use io.Copy for streaming.
		io.Copy(w, videoReader)
		sendViewedMessage(log, videoPath)
	})

	log.Info(`Microservice online!`)
	return http.ListenAndServe(fmt.Sprint(`:`, port), mux)
}

// Attempt to log the watched video to the history microservice upon a view.
func sendViewedMessage(log *slog.Logger, videoPath string) {
	// Create the request body
	messageBody := viewedMessageBody{
		VideoPath: videoPath,
	}

	// Create a new POST request
	req, _ := http.NewRequest(http.MethodPost, `http://history/viewed`, nil)

	// Set the content type header
	req.Header.Set(`Content-Type`, `application/json`)

	// Create a buffer to hold the JSON data
	var jsonBuffer bytes.Buffer

	// Use json.NewEncoder to encode the request body directly into the request
	json.NewEncoder(&jsonBuffer).Encode(messageBody)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(`Failed to send 'viewed' message!`)
		log.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	// Log the success.
	log.Error(`Sent 'viewed' message to history microservice.`)
}
