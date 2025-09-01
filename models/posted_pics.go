package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PostPic struct {
	Id           primitive.ObjectID `json:"id,omitempty"`
	UserID       string             `json:"userID"`
	Title        string             `json:"title,omitempty" validate:"required"`
	Description  string             `json:"description,omitempty"`
	Location     string             `json:"location,omitempty" validate:"required"`
	DateCreated  time.Time          `json:"datecreated,omitempty" validate:"required"`
	Show         bool               `json:"show,omitempty" validate:"required"`
	IsPayPerView bool               `json:"ispayperview,omitempty" validate:"required"`
	PPVPrice     float64            `json:"ppvprice,omitempty" validate:"required"`
	IsDeleted    bool               `json:"isdeleted,omitempty" validate:"required"`
	Tags         []string           `json:"tags,omitempty"`
}

type NewPostPic struct {
	UserID       string    `json:"userID"`
	Title        string    `json:"title,omitempty" validate:"required"`
	Description  string    `json:"description,omitempty"`
	Location     string    `json:"location,omitempty" validate:"required"`
	DateCreated  time.Time `json:"datecreated,omitempty" validate:"required"`
	Show         bool      `json:"show,omitempty" validate:"required"`
	IsPayPerView bool      `json:"ispayperview,omitempty" validate:"required"`
	PPVPrice     float64   `json:"ppvprice,omitempty" validate:"required"`
	IsDeleted    bool      `json:"isdeleted,omitempty" validate:"required"`
	Tags         []string  `json:"tags,omitempty"`
}

func (postpic *PostPic) AddItem(tag string) {
	postpic.Tags = append(postpic.Tags, tag)
}
