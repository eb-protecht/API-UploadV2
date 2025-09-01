package models

import (
	"time"
)

type User struct {
	UserID     string    `json:"_id,omitempty" bson:"_id,omitempty" gorm:"column:_id"`
	UserName   string    `json:"username,omitempty" bson:"username,omitempty" gorm:"column:username"`
	FirstName  string    `json:"firstName,omitempty" bson:"firstName,omitempty" gorm:"column:firstName"`
	LastName   string    `json:"lastName,omitempty" bson:"lastName,omitempty" gorm:"column:lastName"`
	Email      string    `json:"email,omitempty" bson:"email,omitempty" gorm:"column:email"`
	Password   string    `json:"password" bson:"password" gorm:"column:password"`
	City       string    `json:"city" bson:"city" gorm:"column:city"`
	Country    string    `json:"country,omitempty" bson:"country,omitempty" gorm:"column:country"`
	Gender     string    `json:"gender,omitempty" bson:"gender,omitempty" gorm:"column:gender"`
	Phone      string    `json:"phoneNumber,omitempty" bson:"phoneNumber,omitempty" gorm:"column:phoneNumber"`
	DOB        time.Time `json:"dob,omitempty" bson:"dob,omitempty" gorm:"column:dob"`
	Active     bool      `json:"active" bson:"active" gorm:"column:active"`
	ValString  string    `json:"validationString,omitempty" bson:"validationString,omitempty" gorm:"column:val_string"`
	Verified   bool      `json:"verified" bson:"verified" gorm:"column:verified"`
	Categories []string  `json:"categories,omitempty" bson:"categories,omitempty" gorm:"column:categories"`
	MySettings Settings  `json:"mySettings,omitempty" bson:"mySettings,omitempty"`
	OTPSecret  string    `json:"otp_secret,omitempty" bson:"otp_secret,omitempty" gorm:"column:otp_secret"`
	OTPURL     string    `json:"otp_url,omitempty" bson:"otp_url,omitempty" gorm:"column:otp_url"`
	ProfilePic string    `json:"profile_pic" bson:"profile_pic" gorm:"column:profile_pic"`
	CreatedAt  time.Time `json:"created_at,omitempty" bson:"created_at,omitempty" gorm:"column:created_at"`
	UpdatedAt  time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty" gorm:"column:updated_at"`
}

// TableName overrides the default table name used by GORM
func (User) TableName() string {
	return "promotion_users"
}

type Settings struct {
	ProfileVisibleTo     string  `json:"profileVisibleTo,omitempty" bson:"profileVisibleTo,omitempty"`
	ContentVisibleTo     string  `json:"contentVisibleTo,omitempty" bson:"contentVisibleTo,omitempty"`
	CommentNotifications bool    `json:"commentNotifications" bson:"commentNotifications"`
	LikeNotifications    bool    `json:"likeNotifications" bson:"likeNotifications"`
	RepostNotifications  bool    `json:"repostNotifications" bson:"repostNotifications"`
	Subscription         bool    `json:"subscription" bson:"subscription"`
	FollowRequestAction  string  `json:"followRequestAction,omitempty" bson:"followRequestAction,omitempty"`
	RepostRequestAction  string  `json:"repostRequestAction,omitempty" bson:"repostRequestAction,omitempty"`
	SubscriptionPrice    float32 `json:"subscriptionPrice" bson:"subscriptionPrice"`
}

type UpdateUser struct {
	City      string    `json:"city,omitempty" bson:"city,omitempty"`
	Country   string    `json:"country,omitempty" bson:"country,omitempty"`
	Gender    string    `json:"gender,omitempty" bson:"gender,omitempty"`
	Phone     string    `json:"phoneNumber,omitempty" bson:"phoneNumber,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}
