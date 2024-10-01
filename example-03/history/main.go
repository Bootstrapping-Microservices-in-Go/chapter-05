package main

import (
	"context"
	"encoding/json"
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

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error(`run failed`, `err`, err.Error())
		os.Exit(1)
	}
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
	failWithError(log, err, `mongo.Connect`)

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

	// Ensure the viewed queue exists in RabbitMQ;
	// create if necessary.
	viewedMessageQueue, err := ch.QueueDeclare(
		`viewed`, // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	failWithError(log, err, `ch.QueueDeclare`)

	// Create a message retrieval channel now
	// that we know the `viewed` queue exists.
	msgs, err := ch.Consume(
		viewedMessageQueue.Name, // queue
		``,                      // consumer
		false,                   // auto-ack
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)
	failWithError(log, err, `ch.Consume`)

	go func() {
		// Retrieve every message in the viewedMessageQueue.
		// Do we know who sent it?  No, but that's the beauty
		// of it.
		for msg := range msgs {
			var msgBody viewedMessageBody
			bson.Unmarshal(msg.Body, &msgBody)

			// Store msgBody into our collection.
			res, err := collection.InsertOne(context.TODO(), msgBody)
			failWithError(log, err, `collection.InsertOne`)
			log.Info(`collection.InsertOne`, `insertedId`, res.InsertedID)
			// Acknowledge the message receipt so it can be deleted.
			msg.Ack(true)
		}
	}()

	// The viewed handler is no longer necessary since we're pulling from the
	// queue.  But we do need an endpoint that will print our view history.
	mux := http.NewServeMux()
	// That's where /history comes in.
	mux.HandleFunc(`GET /history`, func(w http.ResponseWriter, r *http.Request) {
		// Retrieve a list of limit messages after the first skip many.
		skip, limit := r.FormValue(`skip`), r.FormValue(`limit`)
		skipInt, _ := strconv.Atoi(skip)
		limitInt, _ := strconv.Atoi(limit)
		// Set the find option parameters.
		findOptions := options.Find().
			SetSkip(int64(skipInt)).
			SetLimit(int64(limitInt))

		// Find every entry.  Ignore the first skipInt,
		// retrieve up to limitInt.
		cursor, err := collection.Find(context.TODO(), bson.D{}, findOptions)
		warnOnNonFatalError(log, err, `/history.collection.Find`)

		// Create storage space for our query and retrieve.
		var results []viewedMessageBody
		err = cursor.All(context.TODO(), &results)
		warnOnNonFatalError(log, err, `/history.Cursor.All`)

		// Return the results as a JSON body.
		json.NewEncoder(w).Encode(results)
	})

	// Start the server.
	log.Info(`Microservice online.`)
	return http.ListenAndServe(fmt.Sprintf(`:%s`, port), mux)
}

// warnOnNonFatalError logs errors without stopping the program.
func warnOnNonFatalError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
	}
}

// failWithError logs the error and stops the program.
func failWithError(log *slog.Logger, err error, msg string) {
	if err != nil {
		log.Error(msg, `error`, err)
		os.Exit(2)
	}
}
