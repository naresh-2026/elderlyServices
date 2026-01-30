package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WeatherRequest struct {
	Season string `json:"season"`
}

type DressResponse struct {
	Season  string   `json:"season"`
	Clothes []string `json:"clothes"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var collection *mongo.Collection

func initMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx,
		options.Client().ApplyURI("mongodb://database:27017"))
	if err != nil {
		log.Fatal(err)
	}

	collection = client.Database("fashionDB").Collection("season_clothes")
	log.Println("MongoDB connected")
}

func getClothesBySeason(season string) (*DressResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result DressResponse
	err := collection.FindOne(ctx, bson.M{"season": season}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()
	log.Println("Client connected")
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Client disconnected")
			break
		}
		var req WeatherRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			conn.WriteJSON(map[string]string{
				"error": "Invalid request",
			})
			continue
		}
		resp, err := getClothesBySeason(req.Season)
		if err != nil {
			conn.WriteJSON(map[string]string{
				"error": "No suggestion found",
			})
			continue
		}
		log.Println("sending cloth suggestions")
		conn.WriteJSON(resp)
	}
}

func main() {
	initMongo()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)
	log.Println("WebSocket server running on ws://localhost:8185/ws")
	log.Fatal(http.ListenAndServe(":8185", nil))
}