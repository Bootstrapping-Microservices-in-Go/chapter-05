package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type viewedMessageBody struct {
	VideoPath string `json:"videoPath"`
}

func main() {
	port := os.Getenv(`PORT`)
	dbhost := os.Getenv(`DBHOST`)
	dbname := os.Getenv(`DBNAME`)

	// Connect to Mongo
	// https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo
	clientOpts := options.Client().
		ApplyURI(dbhost)
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		log.Fatal("failed to connect to MongoDB", err)
	}
	collection := client.Database(dbname).Collection(`history`)

	mux := http.NewServeMux()
	mux.HandleFunc(`POST /viewed`, func(w http.ResponseWriter, r *http.Request) {
		var msgBody viewedMessageBody
		if err := json.NewDecoder(r.Body).Decode(&msgBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}

		collection.InsertOne(context.TODO(), bson.D{{`videoPath`, msgBody}})
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
}
