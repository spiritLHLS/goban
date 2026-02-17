package database

import (
	"github.com/spiritlhl/goban/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

func InitDB() error {
	var err error
	db, err = gorm.Open(sqlite.Open("goban.db"), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移数据库表
	return db.AutoMigrate(
		&models.BiliUser{},
		&models.MonitorTask{},
		&models.MonitorLog{},
		&models.ReportRecord{},
	)
}

func GetDB() *gorm.DB {
	return db
}
