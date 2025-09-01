package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Stream struct {
	Id          primitive.ObjectID `json:"id,omitempty"`
	Title       string             `json:"title,omitempty"`
	FSLocation  string             `json:"fslocation,omitempty" validate:"required"`
	Location    Location           `json:"location"`
	DateCreated time.Time          `json:"datecreated,omitempty" validate:"required"`
	Tags        []string           `json:"tags,omitempty"`
	UserID      string             `json:"userID"`
	IsActive    bool               `json:"isActive"`
}
