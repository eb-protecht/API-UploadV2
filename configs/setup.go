package configs

import (
	"context"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectDB() error {
	if DB != nil {
		return nil // Already connected
	}

	logger := LogWithContext("database", "mongodb-connect")

	client, err := mongo.NewClient(options.Client().ApplyURI(EnvMongoURI()))
	if err != nil {
		logger.Error("Failed to create MongoDB client", "error", err, "uri", EnvMongoURI())
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		logger.Error("Failed to connect to MongoDB", "error", err)
		return err
	}

	// Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		logger.Error("Failed to ping MongoDB", "error", err)
		return err
	}

	DB = client
	logger.Info("Connected to MongoDB successfully", "uri", EnvMongoURI())
	return nil
}

// Client instance
var DB *mongo.Client
var REDIS *redis.Client

// getting database collections
func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	if client == nil {
		panic("MongoDB client is nil - database not connected")
	}

	var dbName string
	
	// GOD HELP US - use local database for users and follows
	if collectionName == "users" || collectionName == "follows" {
		dbName = "eyeCDB"
		Logger.Debug("Using local eyeCDB database", "collection", collectionName)
	} else {
		// Extract database name from MongoDB URI for other collections
		uri := EnvMongoURI()
		Logger.Debug("Getting MongoDB collection", "uri", uri, "collection", collectionName)
		
		parts := strings.Split(uri, "/")
		if len(parts) >= 4 {
			dbName = strings.Split(parts[3], "?")[0]
			Logger.Debug("Extracted database name from URI", "database", dbName)
		} else {
			Logger.Warn("Failed to parse database name from URI, using fallback", "fallback_db", "synapp")
			dbName = "synapp"
		}
	}
	
	collection := client.Database(dbName).Collection(collectionName)
	return collection
}

// func ConnectREDISDB() error {
// 	if REDIS != nil {
// 		return nil // Already connected
// 	}

// 	logger := LogWithContext("database", "redis-connect")

// 	client := redis.NewClient(&redis.Options{
// 		Addr:     RedisURL(),
// 		Password: "",
// 		DB:       0,
// 	})

// 	pong, err := client.Ping().Result()
// 	if err != nil {
// 		logger.Error("Failed to connect to Redis", "error", err, "address", RedisURL())
// 		return err
// 	}

// 	REDIS = client
// 	logger.Info("Connected to Redis successfully", "address", RedisURL(), "ping_response", pong)
// 	return nil
// }
