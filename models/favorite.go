package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Favorite struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID      string             `json:"userID" bson:"userid"`
	Albums      []Album            `json:"albums" bson:"albums"`
	DateCreated time.Time          `json:"datecreated" bson:"datecreated"`
}

type Album struct {
	Content     []string  `json:"content" bson:"content"`
	Title       string    `json:"title" bson:"title"`
	DateCreated time.Time `json:"datecreated" bson:"datecreated"`
}
