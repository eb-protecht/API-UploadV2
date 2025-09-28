package controllers

import (
	//"context"
	"context"
	"encoding/json"
	"log"
	"time"

	//"encoding/json"

	//"log"

	//"time"
	//"upload-service/configs"
	"upload-service/configs"
	"upload-service/models"

	"go.mongodb.org/mongo-driver/bson"
	//"go.mongodb.org/mongo-driver/bson"
	//"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	VIDEO               = "video"
	PIC                 = "pic"
	STREAM              = "stream"
	TEXT                = "text"
	CONTENT_BY_USER     = "contentByUser:"
	USER                = "user:"
	TRANSCODING_DONE    = "done"
	TRANSCODING_PENDING = "pending"
	TRANSCODING_FAILED  = "failed"
)

func sendNotificationWithData(userID, initiatorID, message, contentID string, notificationType models.NotificationType, ctx context.Context) {
	notificationData := models.Notification{
		Type:        notificationType,
		UserID:      userID,
		InitiatorID: initiatorID,
		Message:     message,
		ContentID:   contentID,
		Status:      "pending",
		DateCreated: time.Now(),
		UpdatedAt:   time.Now(),
	}
	jsonData, err := json.Marshal(notificationData)
	if err != nil {
		log.Println("Error marshaling data:", err)
		return
	}
	configs.REDIS.Publish(configs.NOTIFICATIONCHANNEL(), jsonData)
}

func sendLiveStartedNotification(userID string, contentID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	followings := []models.Follow{}
	cur, err := followsCollection.Find(ctx, bson.M{"following": userID})
	if err != nil {
		return
	}
	if err := cur.All(ctx, &followings); err != nil {
		return
	}
	for _, v := range followings {
		sendNotificationWithData(v.Follower, userID, "started Live Streaming 👁", contentID, models.LiveStreamingNotification, ctx)
	}
}

// for REDIS
/* func insertInREDISGetContentByUserID(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sortByDateCreated := options.Find()
	sortByDateCreated.SetSort(bson.D{{Key: "datecreated", Value: -1}})
	filterByUserID := bson.M{"$or": []interface{}{
		bson.M{"userid": userID},
		bson.M{"reposter": userID},
	}, "isdeleted": false, "show": true}
	var content []models.Content
	cur, err := contentCollection.Find(ctx, filterByUserID, sortByDateCreated)
	if err != nil {
		fmt.Println("error getting videos for user", userID)
		return
	}
	if err := cur.All(ctx, &content); err != nil {
		if err != nil {
			fmt.Println("error decoding videos for user", userID)
			return
		}
	}
	fmt.Println("looking for pics")
	var returnContent []models.Content
	for _, c := range content {
		var returnc models.Content = c
		if c.Type == PIC {
			fmt.Println("getting pic for " + c.Title + " at location " + c.Location)
			getThumbnail(&returnc)
		} else if c.Type == VIDEO {
			fmt.Println("getting pic for " + c.Title + " at location " + c.Location)
			getVideoThumbnail(&returnc)
		}
		returnContent = append(returnContent, returnc)
	}

	var returned []interface{}
	for _, v := range returnContent {
		returned = append(returned, v)
	}
	jsonData, err := json.Marshal(returned)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}
	configs.REDIS.Set(CONTENT_BY_USER+userID, jsonData, 0)
}

func insertUserInREDIS(userID string, user models.User) {
	jsonData, err := json.Marshal(user)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}
	configs.REDIS.Set(USER+userID, jsonData, 0)
} */
