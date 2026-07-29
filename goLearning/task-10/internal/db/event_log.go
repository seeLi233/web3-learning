package db

import (
	"task10/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EventLog 数据库模型（对应 event_logs 表）
type EventLog struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	TxHash      string    `gorm:"column:tx_hash;type:varchar(66);not null;index:idx_tx_log,unique"`
	BlockNumber uint64    `gorm:"column:block_number;not null;index"`
	LogIndex    uint      `gorm:"column:log_index;not null;index:idx_tx_log,unique"` // tx_hash + log_index 联合唯一
	Contract    string    `gorm:"column:contract;type:varchar(42);not null"`
	EventName   string    `gorm:"column:event_name;type:varchar(64);not null; default:Transfer"`
	Topic0      string    `gorm:"colmun:topic0;type:varchar(66);not null"`
	FromAddr    string    `gorm:"column:from_addr;type:varchar(42);index"`
	ToAddr      string    `gorm:"column:to_addr;type:varchar(42);index"`
	Value       string    `gorm:"column:value;type:varchar(78)"` // uint256 max
	RawData     string    `gorm:"column:raw_data;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (EventLog) TableName() string {
	return "event_logs"
}

// FromTransferEvent 从 model.TransferEvent 转为 DB 模型
func FromTransferEvent(e *model.TransferEvent) *EventLog {
	return &EventLog{
		TxHash:      e.TxHash,
		BlockNumber: e.BlockNumber,
		LogIndex:    e.LogIndex,
		Contract:    e.Contract,
		EventName:   "Transfer",
		Topic0:      "", // 如果需要可以存
		FromAddr:    e.From,
		ToAddr:      e.To,
		Value:       e.Value.String(),
		RawData:     e.RawData,
	}
}

// SaveEvent 保存事件（Upsert: 冲突则忽略）
func SaveEvent(db *gorm.DB, event *EventLog) (bool, error) {
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tx_hash"}, {Name: "log_index"}},
		DoNothing: true, // 冲突就跳过，不报错
	}).Create(event)

	if result.Error != nil {
		return false, result.Error
	}

	// RowsAffected > 0 说明是新记录
	return result.RowsAffected > 0, nil
}

// GetLastProcessedBlock 获取已处理的最大区块号
func GetLastProcessedBlock(db *gorm.DB) (uint64, error) {
	var maxBlock uint64
	result := db.Model(&EventLog{}).Select("COALESCE(MAX(block_number), 0)").Scan(&maxBlock)
	if result.Error != nil {
		return 0, result.Error
	}
	return maxBlock, nil
}
