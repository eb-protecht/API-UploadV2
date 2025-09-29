package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Feedback struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Email       string             `json:"email,omitempty" bson:"email,omitempty"`
	Phone       string             `json:"phone,omitempty" bson:"phone,omitempty"`
	Comment     string             `json:"comment" bson:"comment"`
	DateCreated time.Time          `json:"date_created" bson:"date_created"`
}

type FeedbackRequest struct {
	Email   string `json:"email,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Comment string `json:"comment" validate:"required"`
}
