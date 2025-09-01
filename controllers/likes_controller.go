package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"upload-service/configs"
	"upload-service/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var likesCollection *mongo.Collection = configs.GetCollection(configs.DB, "likes")

func Like() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		vars := mux.Vars(r)
		userID := vars["UserID"]
		likedContent := vars["LikedContent"]
		exists := likesCollection.FindOne(ctx, bson.M{"userid": userID, "likedcontent": likedContent})
		existingLike := models.Like{}
		if err := exists.Decode(&existingLike); err != nil {
			if err != mongo.ErrNoDocuments {
				errorResponse(rw, err, 200)
				return
			}
			like := models.Like{
				UserID:       userID,
				LikedContent: likedContent,
				DateCreated:  time.Now(),
			}
			res, err := likesCollection.InsertOne(ctx, like)
			if err != nil {
				errorResponse(rw, err, 200)
				return
			}
			oCID, err := primitive.ObjectIDFromHex(likedContent)
			if err != nil {
				fmt.Println("couldn't get content from ", likedContent)
				errorResponse(rw, err, 200)
				return
			}
			content := models.Content{}
			err = contentCollection.FindOne(ctx, bson.M{"_id": oCID}).Decode(&content)
			if err != nil {
				fmt.Println("couldn't get decode content from ", oCID)
				errorResponse(rw, err, 200)
				return
			}
			sendNotificationWithData(content.UserID, userID, "liked your post", likedContent, models.LikeNotification, ctx)
			successResponse(rw, res.InsertedID)
			return
		}
		delRes, err := likesCollection.DeleteOne(ctx, bson.M{"_id": existingLike.ID})
		if err != nil {
			errorResponse(rw, err, 200)
			return
		}
		successResponse(rw, delRes.DeletedCount)
	}
}
