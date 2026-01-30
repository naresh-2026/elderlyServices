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
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSRequest struct {
	Type string `json:"type"`    
	Food string `json:"food,omitempty"`
}

type Food struct {
	Food     string  `json:"food" bson:"food"`
	Calories int     `json:"calories" bson:"calories"`
	Protein  float64 `json:"protein" bson:"protein"`
	Carbs    float64 `json:"carbs" bson:"carbs"`
	Fat      float64 `json:"fat" bson:"fat"`
}

type HealthyMeal struct {
	MealType string   `json:"meal_type" bson:"meal_type"`
	Items    []string `json:"items" bson:"items"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

var foodCollection *mongo.Collection
var mealsCollection *mongo.Collection

func initMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx,
		options.Client().ApplyURI("mongodb://database:27017"),
		//options.Client().ApplyURI("mongodb://localhost:27017"),
	)
	if err != nil {
		log.Fatal("MongoDB connection failed:", err)
	}
	db := client.Database("nutrition_db")
	foodCollection = db.Collection("foods")
	mealsCollection = db.Collection("healthy_meals")

	log.Println("MongoDB connected successfully")
}

func getNutrition(food string) (Food, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result Food
	err := foodCollection.
		FindOne(ctx, bson.M{"food": food}).
		Decode(&result)
	log.Println("Sendin nutrirional value for ",food)
	return result, err
}

func getHealthyMeals() ([]HealthyMeal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cursor, err := mealsCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var meals []HealthyMeal
	if err := cursor.All(ctx, &meals); err != nil {
		return nil, err
	}
	log.Println("Sending healthy meal list")
	return meals, nil
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	log.Println("Client connected")
	//meals, err := getHealthyMeals()
	if err != nil {
		return
		//conn.WriteJSON(meals)
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var req WSRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		switch req.Type {
		case "nutrition":
			log.Println("Nutrition ",req.Food)
			res, err := getNutrition(req.Food)
			if err == mongo.ErrNoDocuments {
				conn.WriteJSON(ErrorResponse{Error: "Food not found"})
				continue
			}
			if err != nil {
				conn.WriteJSON(ErrorResponse{Error: "Internal server error"})
				continue
			}
			conn.WriteJSON(res)
		case "meals":
			meals, err := getHealthyMeals()
			if err != nil {
				conn.WriteJSON(ErrorResponse{Error: "Failed to load meals"})
				continue
			}
			conn.WriteJSON(meals)
		default:
			log.Println("Request not found!")
		}
	}
}

func main() {
	initMongo()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)
	log.Println("Server running on :8182")
	log.Fatal(http.ListenAndServe(":8182", nil))
}
