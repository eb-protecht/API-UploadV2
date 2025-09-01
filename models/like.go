package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Like struct {
	ID           primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	LikedContent string             `json:"likedcontent" bson:"likedcontent"`
	UserID       string             `json:"userID" bson:"userid"`
	DateCreated  time.Time          `json:"datecreated" bson:"datecreated"`
}
