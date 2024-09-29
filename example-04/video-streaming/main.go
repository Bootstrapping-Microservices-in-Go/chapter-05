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
	contentLength = `Content-Length`
	contentType   = `Content-Type`
)

type viewedMessageBody struct {
	VideoPath string `json:"videoPath" bson:"videoPath"`
}

func run(log *slog.Logger) error {
	port, found := os.LookupEnv(`PORT`)
	if !found {
		fmt.Errorf(`Please specify the port number for the HTTP server with the environment variable PORT.`)
	}

	rabbit := os.Getenv(`RABBIT`)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failWithError(log, err, `Failed to connect to RabbitMQ`)
	defer conn.Close()

	// Now we need to connect to the queue, consume messages.
	ch, err := conn.Channel()
	failWithError(log, err, `Failed to open a channel`)
	defer ch.Close()

	err = ch.ExchangeDeclare(
		`Viewed`,
		`fanout`,
		true,
		false,
		false,
		false,
		nil)
	failWithError(log, err, `Failed to declare an exchange`)

	q, err := ch.QueueDeclare(
		``,    // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failWithError(log, err, `Failed to declare a queue`)

	ch.QueueBind(q.Name, ``, `Viewed`, false, nil)

	mux := http.NewServeMux()
	mux.HandleFunc(`GET /video`, func(w http.ResponseWriter, r *http.Request) {
		videoPath := `./videos/SampleVideo_1280x720_1mb.mp4`
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
		w.Header().Add(contentType, `video/mp4`)
		// use io.Copy for streaming.
		io.Copy(w, videoReader)
		sendViewedMessage(log, videoPath, ch, &q)
	})

	log.Info(`Microservice online!`)
	return http.ListenAndServe(fmt.Sprint(`:`, port), mux)
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

	err = channel.Publish(`Viewed`, queue.Name, false, false, amqp.Publishing{
		ContentType: `application/bson`,
		Body:        payload,
	})

	failWithError(log, err, `Unable to publish to RabbitMQ channel`)
}

func failWithError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
		os.Exit(2)
	}
}
