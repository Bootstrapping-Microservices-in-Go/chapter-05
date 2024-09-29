package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type viewedMessageBody struct {
	VideoPath string `json:"videoPath" bson:"videoPath"`
}

func run(log *slog.Logger) error {
	port := os.Getenv(`PORT`)
	dbhost := os.Getenv(`DBHOST`)
	dbname := os.Getenv(`DBNAME`)
	rabbit := os.Getenv(`RABBIT`)

	// Connect to Mongo
	// https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo
	clientOpts := options.Client().
		ApplyURI(dbhost)
	client, err := mongo.Connect(context.TODO(), clientOpts)
	failWithError(log, err, `mongo.Connection`)

	collection := client.Database(dbname).Collection(`history`)
	defer client.Disconnect(context.TODO())

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
		``,     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failWithError(log, err, `ch.Consume`)

	go func() {
		for d := range msgs {
			var msgBody viewedMessageBody
			bson.Unmarshal(d.Body, &msgBody)

			// Add to Mongo
			res, err := collection.InsertOne(context.TODO(), msgBody)
			failWithError(log, err, `collection.InsertOne`)
			slog.Info(`collection.InsertOne`, `insertedId`, res.InsertedID)
			d.Ack(true)
		}
	}()

	// The viewed handler is no longer necessary since we're ulling from the
	// queue.  But we do need an endpoint that will print our view history.
	mux := http.NewServeMux()
	mux.HandleFunc(`GET /history`, func(w http.ResponseWriter, r *http.Request) {
		skip, limit := r.FormValue(`skip`), r.FormValue(`limit`)
		skipInt, _ := strconv.Atoi(skip)
		limitInt, _ := strconv.Atoi(limit)
		w.Header().Add(`Content-Type`, `plain/text`)
		findOptions := options.Find().
			SetSkip(int64(skipInt)).
			SetLimit(int64(limitInt))

		cursor, err := collection.Find(context.TODO(), bson.D{}, findOptions)
		warnOnNonFatalError(log, err, `/history.collection.Find`)

		var results []viewedMessageBody

		err = cursor.All(context.TODO(), &results)
		warnOnNonFatalError(log, err, `/history.Cursor.All`)

		for _, result := range results {
			log.Info(`cursor.All`, `videoPath`, result.VideoPath)
		}
	})

	log.Info(`Microservice online!`)
	return http.ListenAndServe(fmt.Sprintf(`:%s`, port), mux)
}

func warnOnNonFatalError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
	}
}

func failWithError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
		os.Exit(2)
	}
}
