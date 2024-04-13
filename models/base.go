package models

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// Init models
func Init() {

	conn, err := gorm.Open(sqlite.Open("orders.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})

	if err != nil {
		panic("failed to connect database")
	}

	db = conn

	err = db.AutoMigrate(&PoolConfig{})
	if err != nil {
		panic(err)
	}
}

// GetDB for other methods
func GetDB() *gorm.DB {
	return db
}

// BaseModel for other models
type BaseModel struct {
	gorm.Model
}

// GetID to find id
func (base *BaseModel) GetID() uint { return base.ID }
