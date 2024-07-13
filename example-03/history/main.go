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

func failOnError(err error, msg string) {
	if err != nil {
		log.Printf("%s: %s", msg, err)
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
	failOnError(err, "Failed to connect to MongoDB")

	collection := client.Database(dbname).Collection(`history`)
	defer client.Disconnect(context.TODO())

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// Now we need to connect to the queue, consume messages.
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"viewed", // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		for d := range msgs {
			var msgBody viewedMessageBody
			bson.Unmarshal(d.Body, &msgBody)

			// Add to Mongo
			res, err := collection.InsertOne(context.TODO(), msgBody)
			failOnError(err, `Failed on insertion`)
			log.Println(res.InsertedID)
		}
	}()

	// The viewed handler is no longer necessary since we're ulling from the
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
		failOnError(err, `fail on find`)

		var results []struct {
			VideoPath string
		}

		err = cursor.All(context.TODO(), &results)
		failOnError(err, `fail on cursor decoding`)

		for _, result := range results {
			log.Println(result)
		}
	})

	http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
}
