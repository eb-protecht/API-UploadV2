package configs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectDB() *mongo.Client {
	client, err := mongo.NewClient(options.Client().ApplyURI(EnvMongoURI()))

	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	//ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB")
	return client
}

// Client instance
var DB *mongo.Client = ConnectDB()
var REDIS *redis.Client

// getting database collections
func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	// Extract database name from MongoDB URI
	// URI format: mongodb://user:pass@host:port/database?options
	uri := EnvMongoURI()
	log.Printf("GetCollection: MongoDB URI: %s", uri)
	// Simple parsing to extract database name
	parts := strings.Split(uri, "/")
	if len(parts) >= 4 {
		dbName := strings.Split(parts[3], "?")[0] // Remove query parameters
		log.Printf("GetCollection: extracted database name: %s, collection: %s", dbName, collectionName)
		collection := client.Database(dbName).Collection(collectionName)
		return collection
	}
	// Fallback to hardcoded name if parsing fails
	log.Printf("GetCollection: failed to parse database name from URI, using fallback 'EyeCDB', collection: %s", collectionName)
	collection := client.Database("EyeCDB").Collection(collectionName)
	return collection
}

func ConnectREDISDB() {
	client := redis.NewClient(&redis.Options{
		Addr:     RedisURL(),
		Password: "",
		DB:       0,
	})
	pong, err := client.Ping().Result()
	if err != nil {
		fmt.Println("Error connecting to Redis:", err)
		return
	}
	fmt.Println("Redis ping response:", pong)
	REDIS = client
}
