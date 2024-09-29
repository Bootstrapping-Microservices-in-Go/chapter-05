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
	// dbhost := os.Getenv(`DBHOST`)
	// dbname := os.Getenv(`DBNAME`)
	rabbit := os.Getenv(`RABBIT`)

	// Connect to Mongo
	// // https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo
	// clientOpts := options.Client().
	// 	ApplyURI(dbhost)
	// client, err := mongo.Connect(context.TODO(), clientOpts)
	// failWithError(log, err, "Failed to connect to MongoDB")

	// collection := client.Database(dbname).Collection(`history`)
	// defer client.Disconnect(context.TODO())

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failWithError(log, err, `amqp.Dial`)
	defer conn.Close()

	// Now we need to connect to the queue, consume messages.
	ch, err := conn.Channel()
	failWithError(log, err, `conn.Channel`)
	defer ch.Close()

	err = ch.ExchangeDeclare(
		`Viewed`,
		`fanout`,
		true,
		false,
		false,
		false,
		nil)
	failWithError(log, err, `ch.ExchangeDeclare`)

	q, err := ch.QueueDeclare(
		``,    // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failWithError(log, err, `ch.QueueDeclare`)

	// Bind the queue to the exchange.
	err = ch.QueueBind(
		q.Name,
		``,
		`Viewed`,
		false,
		nil,
	)
	failWithError(log, err, `ch.QueueBind`)

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failWithError(log, err, "Failed to register a consumer")

	go func() {
		for d := range msgs {
			var msgBody viewedMessageBody
			bson.Unmarshal(d.Body, &msgBody)

			log.Info(`'viewed' message ack.`)
		}
	}()

	mux := http.NewServeMux()
	log.Info(`Microservice online!`)
	return http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
}

func failWithError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
		os.Exit(2)
	}
}
