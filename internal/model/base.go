package model

import "time"

// BaseColumns 是所有 RBAC 表统一的公共列。
//   status: 0=正常 1=禁用
//   is_del: 0=正常 1=已删除（软删除）
//   c_by / u_by: 创建/修改者ID
type BaseColumns struct {
	Remark    string    `gorm:"size:500;not null;default:''" json:"remark"`
	Status    int8      `gorm:"not null;default:0" json:"status"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	IsDel     int8      `gorm:"column:is_del;not null;default:0" json:"isDel"`
	CBy       uint64    `gorm:"column:c_by;not null;default:0" json:"cBy"`
	UBy       uint64    `gorm:"column:u_by;not null;default:0" json:"uBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
