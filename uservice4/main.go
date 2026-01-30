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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var userCollection *mongo.Collection

type WSMessage struct {
	Action string          `json:"action"`
	UserID string          `json:"user_id"`
	Data   json.RawMessage `json:"data,omitempty"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		var wsMsg WSMessage
		json.Unmarshal(msg, &wsMsg)

		switch wsMsg.Action {
		case "update_user":
			updateUser(conn, wsMsg)
		case "add_medication":
			addMedication(conn, wsMsg)
		case "get_profile":
			getProfile(conn, wsMsg)
		case "get_medications":
			getMedications(conn, wsMsg)
		default:
			log.Println("No case matched. Bad request")
		}
	}
}

func updateUser(conn *websocket.Conn, msg WSMessage) {
	log.Println("updating user")
	var update bson.M
	json.Unmarshal(msg.Data, &update)
	opts := options.Update().SetUpsert(true)
	_, err := userCollection.UpdateOne(
		context.TODO(),
		bson.M{"_id": msg.UserID},
		bson.M{"$set": update},
		opts,
	)
	if err != nil {
		conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	log.Println("user is updated")
	conn.WriteJSON(map[string]string{"status": "user upserted"})
}

func addMedication(conn *websocket.Conn, msg WSMessage) {
	var payload struct {
		Medication string `json:"medication"`
	}
	json.Unmarshal(msg.Data, &payload)
	_, err := userCollection.UpdateOne(
		context.TODO(),
		bson.M{"_id": msg.UserID},
		bson.M{"$push": bson.M{"previous_medications": payload.Medication}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	conn.WriteJSON(map[string]string{"status": "medication added"})
}

func getProfile(conn *websocket.Conn, msg WSMessage) {
	var result struct {
		ID     string `bson:"_id" json:"id"`
		Name   string `bson:"name" json:"name"`
		Age    int    `bson:"age" json:"age"`
		Gender string `bson:"gender" json:"gender"`
	}
	err := userCollection.FindOne(
		context.TODO(),
		bson.M{"_id": msg.UserID},
		options.FindOne().SetProjection(bson.M{
			"name":   1,
			"age":    1,
			"gender": 1,
		}),
	).Decode(&result)
	if err != nil {
		conn.WriteJSON(map[string]string{"error": "user not found"})
		return
	}
	log.Println("sending info")
	conn.WriteJSON(result)
}

func getMedications(conn *websocket.Conn, msg WSMessage) {
	var result struct {
		Medications []string `bson:"previous_medications" json:"previous_medications"`
	}
	err := userCollection.FindOne(
		context.TODO(),
		bson.M{"_id": msg.UserID},
		options.FindOne().SetProjection(bson.M{
			"previous_medications": 1,
		}),
		).Decode(&result)
	if err != nil {
		conn.WriteJSON(map[string]string{"error": "user not found"})
		return
	}
	log.Println("returning medical history")
	conn.WriteJSON(result)
}

func mongoInit(){
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://database:27017"))
	if err != nil {
		log.Fatal(err)
	}
	userCollection = client.Database("medical").Collection("users")
}

func main() {
	mongoInit()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)
	log.Println("WebSocket server running on :8183")
	http.ListenAndServe(":8183", nil)
}
