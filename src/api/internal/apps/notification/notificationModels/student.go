package notificationModels

import "time"

// StudentNotification 保存学生通知，与组织账号的通知编号及接收人独立。
type StudentNotification struct {
	ID          uint64 `gorm:"primaryKey"`
	WorkspaceID string
	StudentID   uint64
	SourceID    string
	Title       string
	Content     string
	URL         string
	ReadAt      *time.Time
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// StudentDelivery 由业务事务提供，不能通过通知接口指定接收人。
type StudentDelivery struct {
	WorkspaceID string
	StudentID   uint64
	SourceID    string
	Title       string
	Content     string
	URL         string
}
