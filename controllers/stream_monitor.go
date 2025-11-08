package controllers // or services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"upload-service/configs"
	"upload-service/models"
)

func MonitorLiveStreams() {
	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	fmt.Println("Live stream monitor started...")

	for range ticker.C {
		ensureAllStream()
	}
}

func ensureAllStream() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find all streams that could potentially be live createdd recently, not ended, not deleted
	filter := bson.M{
		"type":      TYPE_STREAM,
		"isdeleted": false,
		"$or": []bson.M{
			{"stream_ended": bson.M{"$exists": false}}, 
			{"stream_ended": nil},                      
		},
		"datecreated": bson.M{
			"$gte": time.Now().Add(-24 * time.Hour),
		},
	}

	cursor, err := getContentCollection().Find(ctx, filter)
	if err != nil {
		fmt.Println("Error finding streams:", err)
		return
	}
	defer cursor.Close(ctx)

	var streams []models.Content
	if err = cursor.All(ctx, &streams); err != nil {
		fmt.Println("Error decoding streams:", err)
		return
	}

	fmt.Printf("Checking %d potential live streams...\n", len(streams))

	for _, stream := range streams {
		checkSingleStream(stream)
	}
}

func checkSingleStream(stream models.Content) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	streamingServerIP := configs.EnvStreamingServer()
	hlsURL := fmt.Sprintf("http://%s/hls/%s.m3u8", streamingServerIP, stream.StreamKey)

	isLive := checkHLSExists(hlsURL)

	// If status changed, update database
	if isLive != stream.IsLive {
		fmt.Printf("Stream %s status changed: %v -> %v\n", stream.StreamKey, stream.IsLive, isLive)

		update := bson.M{}

		if isLive {
			// Stream just went live
			now := time.Now()
			update = bson.M{"$set": bson.M{
				"is_live":        true,
				"stream_started": now,
			}}

			fmt.Printf("Stream %s is now LIVE!\n", stream.StreamKey)

			// Send notifications to followers
			go func() {
				contentID := stream.Id.Hex()
				sendLiveStartedNotification(stream.UserID, contentID)
			}()

		} else if stream.IsLive {
			// Stream just ended
			now := time.Now()
			update = bson.M{"$set": bson.M{
				"is_live":      false,
				"stream_ended": now,
			}}

			fmt.Printf("⚫ Stream %s has ENDED\n", stream.StreamKey)
		}

		// Update database
		if len(update) > 0 {
			_, err := getContentCollection().UpdateOne(
				ctx,
				bson.M{"_id": stream.Id},
				update,
			)
			if err != nil {
				fmt.Println("Error updating stream:", err)
			}
		}
	}
}

// Check if HLS playlist file exists
func checkHLSExists(hlsURL string) bool {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(hlsURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}