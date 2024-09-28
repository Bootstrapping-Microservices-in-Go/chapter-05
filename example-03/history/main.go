package main

import (
	"context"
	"fmt"
	"log"
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

func failWithError(err error, msg string, fatal bool) {
	if err != nil {
		log.Printf("%s: %s", msg, err)
	}
	if fatal {
		os.Exit(2)
	}
}

func main() {
	port := os.Getenv(`PORT`)
	dbhost := os.Getenv(`DBHOST`)
	dbname := os.Getenv(`DBNAME`)
	rabbit := os.Getenv(`RABBIT`)

	// Connect to Mongo
	// https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo
	clientOpts := options.Client().
		ApplyURI(dbhost)
	client, err := mongo.Connect(context.TODO(), clientOpts)
	failWithError(err, "Failed to connect to MongoDB", true)

	collection := client.Database(dbname).Collection(`history`)
	defer client.Disconnect(context.TODO())

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failWithError(err, "Failed to connect to RabbitMQ", true)
	defer conn.Close()

	// Now we need to connect to the queue, consume messages.
	ch, err := conn.Channel()
	failWithError(err, "Failed to open a channel", true)
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"viewed", // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	failWithError(err, "Failed to declare a queue", true)

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failWithError(err, "Failed to register a consumer", true)

	go func() {
		for d := range msgs {
			var msgBody viewedMessageBody
			bson.Unmarshal(d.Body, &msgBody)

			// Add to Mongo
			res, err := collection.InsertOne(context.TODO(), msgBody)
			failWithError(err, `Failed on insertion`, false)
			log.Println(res.InsertedID)
			d.Ack(true)
		}
	}()

	// The viewed handler is no longer necessary since we're pulling from the
	// queue.  But we do need an endpoint that will print our view history.
	mux := http.NewServeMux()
	mux.HandleFunc(`GET /history`, func(w http.ResponseWriter, r *http.Request) {
		skip, limit := r.FormValue(`skip`), r.FormValue(`limit`)
		skipInt, _ := strconv.Atoi(skip)
		limitInt, _ := strconv.Atoi(limit)
		w.Header().Add("Content-Type", "plain/text")
		findOptions := options.Find().
			SetSkip(int64(skipInt)).
			SetLimit(int64(limitInt))

		cursor, err := collection.Find(context.TODO(), bson.D{}, findOptions)
		failWithError(err, `fail on find`, false)

		var results []struct {
			VideoPath string
		}

		err = cursor.All(context.TODO(), &results)
		failWithError(err, `fail on cursor decoding`, false)

		for _, result := range results {
			log.Println(result)
		}
	})

	http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
}
