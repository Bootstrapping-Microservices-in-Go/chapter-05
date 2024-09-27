package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	contentLength = "Content-Length"
	contentType   = "Content-Type"
)

type viewedMessageBody struct {
	VideoPath string `json:"videoPath" bson:"videoPath"`
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
		// use json.NewDecoder().Decode() to get videoPath
		var messageBody viewedMessageBody
		err := json.NewDecoder(r.Body).Decode(&messageBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// insertOne in history collection.
		_, err = collection.InsertOne(r.Context(), bson.M{`videoPath`: messageBody.VideoPath})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// print Added video ${videoPath} to history.
		log.Printf(`Added video %s to history.`, messageBody.VideoPath)

		w.WriteHeader(http.StatusOK)

		// Send a POST request to http://history/viewed
		// JSON-encoded.  On failure: Failed to send 'viewed' message!
		// On success: Sent 'viewed' message to history microservice.
	})

	mux.HandleFunc(`GET /history`, func(w http.ResponseWriter, r *http.Request) {
		skip := r.FormValue(`skip`)
		skipInt, err := strconv.Atoi(skip)
		if err != nil {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		limit := r.FormValue(`limit`)
		limitInt, err := strconv.Atoi(limit)
		if err != nil {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}

		findOptions := options.Find()
		findOptions.SetSkip(int64(skipInt))
		findOptions.SetLimit(int64(limitInt))

		cursor, err := collection.Find(context.Background(), bson.D{}, findOptions)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.Background())

		var history []viewedMessageBody
		if err := cursor.All(context.Background(), &history); err != nil {
			log.Fatal(err)
		}

		json.NewEncoder(w).Encode(history)

		// Send a GET request to http://history/history
		// skip, limit parameters.
		// Return as json.NewEncoder().Encode(history)
		// return 200.
	})

	log.Println(`Microservice online!`)
	http.ListenAndServe(fmt.Sprintf(":%s", port), mux)
}
