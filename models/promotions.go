package models

import (
	"time"
)

type Promotions struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                // ID as an auto-incrementing bigint
	ContentID      string    `gorm:"column:content_id;type:varchar(24)" json:"content_id"`        // Content ID as VARCHAR(24)
	Amount         float64   `gorm:"column:amount;type:double precision" json:"amount"`           // Double precision for the amount
	TargetGender   string    `gorm:"column:target_gender;type:char(1)" json:"target_gender"`      // Char(1) for gender
	StartDate      time.Time `gorm:"column:start_date;type:date" json:"start_date"`               // Date for the start date
	EndDate        time.Time `gorm:"column:end_date;type:date" json:"end_date"`                   // Date for the end date
	StartTargetAge int64     `gorm:"column:start_target_age;type:bigint" json:"start_target_age"` // Bigint for start age range
	EndTargetAge   int64     `gorm:"column:end_target_age;type:bigint" json:"end_target_age"`     // Bigint for end age range
}

// TableName sets the custom table name for the Promotions struct
func (Promotions) TableName() string {
	return "promotions"
}
