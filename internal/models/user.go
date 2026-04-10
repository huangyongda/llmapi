package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID              int64          `gorm:"primaryKey" json:"id"`
	Username        string         `gorm:"size:100;uniqueIndex;not null" json:"username"`
	PasswordHash    string         `gorm:"size:255;not null" json:"-"`
	RequestLimit    int            `gorm:"default:0" json:"request_limit"`
	RequestCount    int            `gorm:"default:0" json:"request_count"`
	IsAdmin         bool           `gorm:"default:false" json:"is_admin"`
	Level           int            `gorm:"default:1" json:"level"` // 1=普通用户 2=高速用户
	UseGml          int            `gorm:"default:-1" json:"use_gml"` // 1=是 -1=否
	UseKimi         int            `gorm:"default:-1" json:"use_kimi"` // 1=是 -1=否
	ExpiresAt       *time.Time     `gorm:"index" json:"expires_at"`
	Remark          string         `gorm:"size:500" json:"remark"`
	HasWeeklyLimit  int            `gorm:"default:-1" json:"has_weekly_limit"` // 1=有周额度限制 -1=无周额度限制
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	APIKeys         []APIKey       `gorm:"foreignKey:UserID" json:"api_keys,omitempty"`
}

func (User) TableName() string {
	return "users"
}

type UserResponse struct {
	ID            int64   `json:"id"`
	Username      string  `json:"username"`
	RequestLimit  int     `json:"request_limit"`
	RequestCount  int     `json:"request_count"`
	IsAdmin       bool    `json:"is_admin"`
	Level         int     `json:"level"`
	UseGml        int     `json:"use_gml"`
	UseKimi       int     `json:"use_kimi"`
	ExpiresAt     *string `json:"expires_at"`
	Remark        string  `json:"remark"`
	HasWeeklyLimit int    `json:"has_weekly_limit"`
	CreatedAt     string  `json:"created_at"`
}

func (u *User) ToResponse() UserResponse {
	var expiresAt *string
	if u.ExpiresAt != nil {
		formatted := u.ExpiresAt.Format("2006-01-02 15:04:05")
		expiresAt = &formatted
	}
	return UserResponse{
		ID:            u.ID,
		Username:      u.Username,
		RequestLimit:  u.RequestLimit,
		RequestCount:  u.RequestCount,
		IsAdmin:       u.IsAdmin,
		Level:         u.Level,
		UseGml:        u.UseGml,
		UseKimi:       u.UseKimi,
		ExpiresAt:     expiresAt,
		Remark:        u.Remark,
		HasWeeklyLimit: u.HasWeeklyLimit,
		CreatedAt:     u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// ActivationUser 激活用户表
type ActivationUser struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	Username       string    `gorm:"size:100;uniqueIndex;not null" json:"username"`
	PasswordHash   string    `gorm:"size:255;not null" json:"-"`
	ValidDays      int       `gorm:"default:0" json:"valid_days"`      // 有效天数
	RequestLimit   int       `gorm:"default:0" json:"request_limit"`  // 最高调用次数
	Level          int       `gorm:"default:1" json:"level"`          // 1=普通用户 2=高速用户
	UseGml         int       `gorm:"default:-1" json:"use_gml"`        // 1=是 -1=否
	UseKimi        int       `gorm:"default:-1" json:"use_kimi"`       // 1=是 -1=否
	HasWeeklyLimit int       `gorm:"default:-1" json:"has_weekly_limit"` // 1=有周额度限制 -1=无周额度限制
	Remarks        string    `gorm:"size:500" json:"remarks"`         // 备注
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (ActivationUser) TableName() string {
	return "activation_users"
}

// ActivationUserResponse 激活用户响应
type ActivationUserResponse struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	ValidDays      int    `json:"valid_days"`
	RequestLimit   int    `json:"request_limit"`
	Level          int    `json:"level"`
	UseGml         int    `json:"use_gml"`
	UseKimi        int    `json:"use_kimi"`
	HasWeeklyLimit int    `json:"has_weekly_limit"`
	Remarks        string `json:"remarks"`
	CreatedAt      string `json:"created_at"`
}

func (a *ActivationUser) ToResponse() ActivationUserResponse {
	return ActivationUserResponse{
		ID:             a.ID,
		Username:       a.Username,
		ValidDays:      a.ValidDays,
		RequestLimit:   a.RequestLimit,
		Level:          a.Level,
		UseGml:         a.UseGml,
		UseKimi:        a.UseKimi,
		HasWeeklyLimit: a.HasWeeklyLimit,
		Remarks:        a.Remarks,
		CreatedAt:      a.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}
