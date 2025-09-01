package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationType int

const (
	FollowRequestNotification NotificationType = iota + 1
	FollowRequestAccepted
	FollowNotification
	SharePostRequestNotification
	SharePostRequestAcceptNotification
	LikeNotification
	CommentNotification
	ReplyNotification
	NewMessageNotification
	RewardNotification
	LiveStreamingNotification
)

type Status int

const (
	Pending Status = iota + 1
	Unread
	Opened
)

type Notification struct {
	ID                  primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Type                NotificationType   `json:"type" bson:"type"`
	UserID              string             `json:"userid" bson:"userid"`
	InitiatorID         string             `json:"initiatorid" bson:"initatorid"` // the person who triggered this notification
	ContentID           string             `json:"contentid" bson:"contentid"`
	Thumb               string             `json:"thumb,omitempty" bson:"thumb,omitempty"` //optional
	Message             string             `json:"message" bson:"message"`
	InitiatorProfilePic string             `json:"initiator_profile_pic,omitempty" bson:"initiator_profile_pic,omitempty"` //optional
	Status              string             `json:"status" bson:"status"`
	DateCreated         time.Time          `json:"datecreated,omitempty" bson:"datecreated,omitempty"`
	UpdatedAt           time.Time          `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
