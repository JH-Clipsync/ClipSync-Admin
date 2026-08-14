package model

import "time"

// User 对应 ClipSync-Server 的 users 表（不在此 AutoMigrate，由 ClipSync-Server 管理）。
// disabled: 0=正常 1=禁用（对应前端传的 status 字段，后端做映射）
type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	Disabled     int8      `gorm:"not null;default:0" json:"disabled"` // 0正常 1禁用
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (User) TableName() string { return "users" }
