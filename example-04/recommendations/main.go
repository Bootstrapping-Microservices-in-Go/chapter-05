package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
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
	// dbhost := os.Getenv(`DBHOST`)
	// dbname := os.Getenv(`DBNAME`)
	rabbit := os.Getenv(`RABBIT`)

	// Connect to Mongo
	// // https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo
	// clientOpts := options.Client().
	// 	ApplyURI(dbhost)
	// client, err := mongo.Connect(context.TODO(), clientOpts)
	// failOnError(err, "Failed to connect to MongoDB")

	// collection := client.Database(dbname).Collection(`history`)
	// defer client.Disconnect(context.TODO())

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbit)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// Now we need to connect to the queue, consume messages.
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		`Viewed`,
		`fanout`,
		true,
		false,
		false,
		false,
		nil)
	failOnError(err, "Failed to declare an exchange")

	q, err := ch.QueueDeclare(
		``,    // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// Bind the queue to the exchange.
	err = ch.QueueBind(
		q.Name,
		``,
		`Viewed`,
		false,
		nil,
	)
	failOnError(err, `Failed to bind queue to exchange`)

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

			log.Println(`Revieved a 'viewed' message.`)
			// d.Ack(false)
		}
	}()

	mux := http.NewServeMux()
	log.Println(`Microservice online!`)
	http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
}
