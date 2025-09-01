package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RepostRequest struct {
	ID            primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Status        string             `json:"status" bson:"status"`
	ContentID     string             `json:"contentid" bson:"contentid"`
	Content       Content            `json:"content" bson:"content"`
	RepostRequest string             `json:"repostRequest" bson:"repostRequest"`
	RequestTo     string             `json:"requestTo" bson:"requestTo"`
	CreatedAt     time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt     time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
