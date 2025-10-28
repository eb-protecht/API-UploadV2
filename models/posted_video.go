package models

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"gorm.io/gorm"
)

type Content struct {
	Id           primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty" gorm:"-"`
	VideoID      string    `json:"video_id,omitempty" bson:"video_id,omitempty"`
	PGId         string             `gorm:"column:_id;type:varchar(24);primaryKey"`
	OriginalID   string             `json:"originalID,omitempty" bson:"originalid,omitempty" gorm:"column:original_id;type:text"`
	Type         string             `json:"type,omitempty" bson:"type" gorm:"column:type;type:text"`
	UserID       string             `json:"userID" bson:"userid" gorm:"column:user_id;type:varchar(24)"`
	Poster       string             `json:"poster" bson:"reposter" gorm:"column:poster;type:text"`
	Posting      string             `json:"posting" bson:"posting" gorm:"column:posting;type:text"`
	Title        string             `json:"title,omitempty" bson:"title,omitempty" gorm:"column:title;type:text"`
	Description  string             `json:"description,omitempty" bson:"description,omitempty" gorm:"column:description;type:text"`
	S3RawKey     string    `json:"s3_raw_key,omitempty" bson:"s3_raw_key,omitempty"`      
	ThumbnailKey string    `json:"thumbnail_key,omitempty" bson:"thumbnail_key,omitempty"` 
	HLSURL       string    `json:"hls_url,omitempty" bson:"hls_url,omitempty"`             
	Status       string    `json:"status,omitempty" bson:"status,omitempty"`         
	Location     string             `json:"location,omitempty" bson:"location" gorm:"column:location;type:text"`
	DateCreated  time.Time          `json:"datecreated,omitempty" bson:"datecreated" gorm:"column:datecreated;type:date"`
	Show         bool               `json:"show,omitempty" bson:"show" gorm:"column:show;type:boolean;default:true"`
	IsPayPerView bool               `json:"ispayperview,omitempty" bson:"ispayperview" gorm:"column:is_payperview;type:boolean;default:false"`
	IsDeleted    bool               `json:"isdeleted,omitempty" bson:"isdeleted" gorm:"column:isdeleted;type:boolean;default:false"`
	PPVPrice     float64            `json:"ppvprice" bson:"ppvprice" gorm:"column:ppvprice;type:float"`
	Tags         []string           `json:"tags" bson:"tags" gorm:"-"`
	Visibility   string             `json:"visibility" bson:"visibility" gorm:"column:visibility;type:text"`
	PgTags       string             `gorm:"column:tags;type:varchar[]"` // Used internally for PostgreSQL
	Transcoding  string             `json:"transcoding,omitempty" bson:"transcoding,omitempty" gorm:"-"`
}

// Before saving the content to PostgreSQL, convert the []string to a PostgreSQL array string
func (c *Content) BeforeCreate(tx *gorm.DB) (err error) {
	c.PgTags = fmt.Sprintf("{%s}", strings.Join(c.Tags, ","))
	c.PGId = c.Id.Hex()
	return
}

// After fetching the content from PostgreSQL, convert the PostgreSQL array string to []string
func (c *Content) AfterFind(tx *gorm.DB) (err error) {
	if c.PgTags != "" {
		c.Tags = strings.Split(strings.Trim(c.PgTags, "{}"), ",")
	}
	oid, err := primitive.ObjectIDFromHex(c.PGId)
	c.Id = oid
	return
}

type ContentBody struct {
	Description string `json:"description,omitempty"`
	Title       string `json:"title,omitempty"`
	Posting     string `json:"posting,omitempty"`
}

type PostVideo struct {
	Id           primitive.ObjectID `json:"id,omitempty"`
	UserID       string             `json:"userID"`
	Title        string             `json:"title,omitempty"`
	Description  string             `json:"description,omitempty" validate:"required"`
	Location     string             `json:"location,omitempty" validate:"required"`
	DateCreated  time.Time          `json:"datecreated,omitempty" validate:"required"`
	Show         bool               `json:"show,omitempty" validate:"required"`
	IsPayPerView bool               `json:"ispayperview,omitempty" validate:"required"`
	IsDeleted    bool               `json:"isdeleted,omitempty" validate:"required"`
	PPVPrice     float64            `json:"ppvprice,omitempty" validate:"required"`
	Tags         []string           `json:"tags,omitempty"`
}
type NewPostVideo struct {
	VideoID      string    `json:"video_id,omitempty" bson:"video_id,omitempty"`
	UserID       string    `json:"userID"`
	Title        string    `json:"title,omitempty"`
	Description  string    `json:"description,omitempty" validate:"required"`
	Location     string    `json:"location,omitempty" validate:"required"`
	S3RawKey     string    `json:"s3_raw_key,omitempty" bson:"s3_raw_key,omitempty"`      
	ThumbnailKey string    `json:"thumbnail_key,omitempty" bson:"thumbnail_key,omitempty"` 
	HLSURL       string    `json:"hls_url,omitempty" bson:"hls_url,omitempty"`             
	Status       string    `json:"status,omitempty" bson:"status,omitempty"`              
	DateCreated  time.Time `json:"datecreated,omitempty" validate:"required"`
	Show         bool      `json:"show,omitempty" validate:"required"`
	IsPayPerView bool      `json:"ispayperview,omitempty" validate:"required"`
	IsDeleted    bool      `json:"isdeleted,omitempty" validate:"required"`
	PPVPrice     float64   `json:"ppvprice,omitempty" validate:"required"`
	Tags         []string  `json:"tags,omitempty"`
}