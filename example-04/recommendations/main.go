package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
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
	port := os.Getenv(`PORT`)
	rabbit := os.Getenv(`RABBIT`)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failWithError(log, err, `amqp.Dial`)
	defer conn.Close()

	// Create a channel to communicate with RabbitMQ.
	ch, err := conn.Channel()
	failWithError(log, err, `conn.Channel`)
	defer ch.Close()

	// Declare an exchange of type "fanout".
	// This exchange will route messages to all queues bound to it,
	// allowing for broadcast messaging to multiple consumers.
	err = ch.ExchangeDeclare(
		`Viewed`,
		`fanout`,
		true,
		false,
		false,
		false,
		nil)
	failWithError(log, err, `ch.ExchangeDeclare`)

	// Declare a queue named "recommendationsQueue".
	// This queue will be used to store messages routed from the "Viewed" exchange.
	recommendationsQueue, err := ch.QueueDeclare(
		`recommendationsQueue`, // name
		false,                  // durable
		false,                  // delete when unused
		false,                  // exclusive
		false,                  // no-wait
		nil,                    // arguments
	)
	failWithError(log, err, `ch.QueueDeclare`)

	// Bind the recommendationsQueue to the exchange.
	err = ch.QueueBind(
		recommendationsQueue.Name, // Queue name to bind,
		``,                        // routing key; not applicable for fanout exchanges
		`Viewed`,                  // Exchange name.
		false,                     // no-wait
		nil,                       // arguments
	)
	failWithError(log, err, `ch.QueueBind`)

	// Create a channel to receive messages sent to our recommendationsQueue.
	msgs, err := ch.Consume(
		recommendationsQueue.Name, // queue
		``,                        // consumer
		false,                     // auto-ack
		false,                     // exclusive
		false,                     // no-local
		false,                     // no-wait
		nil,                       // args
	)
	failWithError(log, err, `ch.Consume`)

	go func() {
		for d := range msgs {
			var msgBody viewedMessageBody
			bson.Unmarshal(d.Body, &msgBody)

			log.Info(`'viewed' message ack.`)
		}
	}()

	mux := http.NewServeMux()
	log.Info(`Microservice online!`)
	// We're simply starting this server as a demo.
	return http.ListenAndServe(fmt.Sprintf(`:%s`, port), mux)
}

func failWithError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
		os.Exit(2)
	}
}
