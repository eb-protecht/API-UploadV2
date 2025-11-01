package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProfilePic struct {
	ID          primitive.ObjectID `json:"id" bson:"_id"`
	UserID      string             `json:"userid"`
	Location    string             `json:"location,omitempty" bson:"location,omitempty"`
	Base64      string             `json:"base64,omitempty" bson:"base64,omitempty"`
	Filename    string             `json:"filename,omitempty" bson:"filename,omitempty"`
	DateCreated time.Time          `json:"datecreated,omitempty" validate:"required"`
	IsCurrent   bool               `json:"iscurrent,omitempty" validate:"required"`
	IsDeleted   bool               `json:"isdeleted,omitempty" validate:"required"`
}
type NewProfilePic struct {
	ID          primitive.ObjectID `json:"id" bson:"_id"`
	UserID      string             `json:"userid"`
	Location    string             `json:"location,omitempty" bson:"location,omitempty"`
	S3RawKey     string    `json:"s3_raw_key,omitempty" bson:"s3_raw_key,omitempty"`   
	Filename    string             `json:"filename,omitempty" bson:"filename,omitempty"` 
	DateCreated time.Time          `json:"datecreated,omitempty" validate:"required"`
	IsCurrent   bool               `json:"iscurrent,omitempty" validate:"required"`
	IsDeleted   bool               `json:"isdeleted,omitempty" validate:"required"`
}
