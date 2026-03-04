package database

import (
	"log"

	"scheduler/internal/domain"

	"gorm.io/gorm"
)

// Seed заполняет базу тестовыми данными, если она пустая
func Seed(db *gorm.DB) {
	var count int64
	db.Model(&domain.Room{}).Count(&count)

	// Если данные уже есть, не дублируем их
	if count > 0 {
		log.Println("Database already seeded. Skipping...")
		return
	}

	log.Println("Seeding database with initial data...")

	// 1. Создаем Аудитории
	rooms := []domain.Room{
		{Name: "A-101 (Лекционная)", Capacity: 60, Type: domain.RoomTypeLecture},
		{Name: "B-202 (Практика)", Capacity: 20, Type: domain.RoomTypePractice},
		{Name: "C-303 (Компьютерный класс)", Capacity: 15, Type: domain.RoomTypeLab},
	}
	db.Create(&rooms)

	// 2. Создаем Временные слоты (допустим, ПН и ВТ, по 2 пары)
	slots := []domain.TimeSlot{
		{Day: domain.DayMonday, PeriodNumber: 1, StartTime: "09:00", EndTime: "10:30"},
		{Day: domain.DayMonday, PeriodNumber: 2, StartTime: "10:40", EndTime: "12:10"},
		{Day: domain.DayTuesday, PeriodNumber: 1, StartTime: "09:00", EndTime: "10:30"},
		{Day: domain.DayTuesday, PeriodNumber: 2, StartTime: "10:40", EndTime: "12:10"},
	}
	db.Create(&slots)

	// 3. Создаем Преподавателей
	instructors := []domain.Instructor{
		{Name: "Иванов И.И."},
		{Name: "Петров П.П."},
	}
	db.Create(&instructors)

	// 4. Создаем Группы
	groups := []domain.Group{
		{Name: "CS-101", Size: 20},
		{Name: "CS-102", Size: 22},
	}
	db.Create(&groups)

	// 5. Создаем Предметы
	subjects := []domain.Subject{
		{Name: "Высшая математика"},
		{Name: "Программирование на Go"},
	}
	db.Create(&subjects)

	// 6. Создаем Занятия (Course Classes) - это то, что мы будем распределять!
	classes := []domain.CourseClass{
		{
			// Потоковая лекция по Математике для обеих групп
			SubjectID: subjects[0].ID, InstructorID: instructors[0].ID,
			Groups:    []*domain.Group{&groups[0], &groups[1]},
			IsLecture: true, RequiredRoomType: domain.RoomTypeLecture, Duration: 1,
		},
		{
			// Практика по Go для 1 группы
			SubjectID: subjects[1].ID, InstructorID: instructors[1].ID,
			Groups:    []*domain.Group{&groups[0]},
			IsLecture: false, RequiredRoomType: domain.RoomTypeLab, Duration: 1,
		},
		{
			// Практика по Go для 2 группы
			SubjectID: subjects[1].ID, InstructorID: instructors[1].ID,
			Groups:    []*domain.Group{&groups[1]},
			IsLecture: false, RequiredRoomType: domain.RoomTypeLab, Duration: 1,
		},
	}
	db.Create(&classes)

	log.Println("Seeding completed successfully!")
}
