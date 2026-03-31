package storage

import "gorm.io/gorm"

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&AdminSession{},
		&PendingAction{},
		&ActionLog{},
	)
}
