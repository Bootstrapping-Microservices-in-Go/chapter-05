package main

import (
	"fmt"
	"io"
	"log"
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

func failWitnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func sendViewedMessage(path string, channel *amqp.Channel, queue *amqp.Queue) {
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
	failWitnError(err, "Unable to publish to RabbitMQ channel")
}

func main() {
	port, found := os.LookupEnv(`PORT`)
	if !found {
		log.Fatal(`Please specify the port number for the HTTP server with the environment variable PORT.`)
	}

	rabbit := os.Getenv(`RABBIT`)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failWitnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// Now we need to connect to the queue, consume messages.
	ch, err := conn.Channel()
	failWitnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"viewed", // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	failWitnError(err, "Failed to declare a queue")

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
		sendViewedMessage(videoPath, ch, &q)
	})

	http.ListenAndServe(fmt.Sprint(":", port), mux)
}
