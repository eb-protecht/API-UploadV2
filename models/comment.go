package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Comment struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	ReplyTo        string             `json:"replyto" bson:"replyto"`
	UserID         string             `json:"userID" bson:"userid"`
	Comment        string             `json:"comment" bson:"comment"`
	ReplyToComment bool               `json:"replytocomment" bson:"replytocomment"`
	IsDeleted      bool               `json:"isdeleted" bson:"isdeleted"`
	DateCreated    time.Time          `json:"datecreated" bson:"datecreated"`
	ContentID      string             `json:"-" bson:"contentid,omitempty"`
}

type CommentBody struct {
	Comment string `json:"comment"`
}
