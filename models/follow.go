package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Follow struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Follower  string             `json:"follower" bson:"follower"`
	Following string             `json:"following" bson:"following"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
