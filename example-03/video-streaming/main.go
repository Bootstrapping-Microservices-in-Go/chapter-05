package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
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
		logger.Error(`run failed`, `err`, err.Error())
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	port, found := os.LookupEnv(`PORT`)
	if !found {
		return fmt.Errorf(`Please specify the port number for the HTTP server with the environment variable PORT.`)
	}

	rabbit := os.Getenv(`RABBIT`)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failWithError(log, err, `amqp.Dial`)
	defer conn.Close()

	// Now we need to connect to the queue, consume messages.
	ch, err := conn.Channel()
	failWithError(log, err, `conn.Channel`)
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"viewed", // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	failWithError(log, err, `ch.QueueDeclare`)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /video", func(w http.ResponseWriter, r *http.Request) {
		videoPath := "./videos/SampleVideo_1280x720_1mb.mp4"
		videoReader, err := os.Open(videoPath)
		if err != nil {
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
		sendViewedMessage(log, videoPath, ch, &q)
	})

	return http.ListenAndServe(fmt.Sprint(":", port), mux)
}

func sendViewedMessage(log *slog.Logger, path string, channel *amqp.Channel, queue *amqp.Queue) {
	// Refactor to send to RabbitMQ.
	body := viewedMessageBody{
		VideoPath: path,
	}

	payload, err := bson.Marshal(body)
	if err != nil {
		return
	}

	err = channel.Publish("", queue.Name, false, false, amqp.Publishing{
		ContentType: `application/bson`,
		Body:        payload,
	})
	failWithError(log, err, `channel.Publish`)
}

func failWithError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
		os.Exit(2)
	}
}
