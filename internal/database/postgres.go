package database

import (
	"log"

	"scheduler/internal/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewConnection(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Автомиграция (замена Alembic для простых случаев)
	// GORM сам создаст таблицы и связи
	err = db.AutoMigrate(
		&domain.TimeSlot{},
		&domain.Room{},
		&domain.Instructor{},
		&domain.Group{},
		&domain.Subject{},
		&domain.CourseClass{},
	)
	if err != nil {
		log.Fatal("Migration failed:", err)
	}

	return db
}
