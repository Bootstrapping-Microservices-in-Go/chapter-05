package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

const (
	contentLength = "Content-Length"
	contentType   = "Content-Type"
)

func main() {
	port, found := os.LookupEnv(`PORT`)
	if !found {
		log.Fatal(`Please specify the port number for the HTTP server with the environment variable PORT.`)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /video", func(w http.ResponseWriter, r *http.Request) {
		log.Print("Found")
		videoPath := "./videos/SampleVideo_1280x720_1mb.mp4"
		videoReader, err := os.Open(videoPath)
		if err != nil {
			log.Print("Not Found")
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

	http.ListenAndServe(fmt.Sprint(":", port), mux)
}
