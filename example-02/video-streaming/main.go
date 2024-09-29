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
		log.Fatal(`Please specify the port number for the HTTP server with the environment variable PORT.`)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /video", func(w http.ResponseWriter, r *http.Request) {
		videoPath := "./videos/SampleVideo_1280x720_1mb.mp4"
		videoReader, err := os.Open(videoPath)
		if err != nil {
			log.Error(`/video.os.Open`, `err`, err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer videoReader.Close()
		videoStats, err := videoReader.Stat()
		if err != nil {
			log.Error(`/videoReader.stat`, `err`, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add(contentLength, strconv.FormatInt(videoStats.Size(), 10))
		w.Header().Add(contentType, "video/mp4")
		// use io.Copy for streaming.
		io.Copy(w, videoReader)
		sendViewedMessage(videoPath)
	})

	http.ListenAndServe(fmt.Sprint(":", port), mux)
}

func sendViewedMessage(path string) {
	body := viewedMessageBody{
		VideoPath: path,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, `http://history/viewed`, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set(contentType, `application/json`)

	client := &http.Client{}
	_, err = client.Do(req)
	log.Print("Viewed")
}
