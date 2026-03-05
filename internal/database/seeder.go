package database

import (
	"log"
	"math/rand"
	"time"

	"scheduler/internal/domain"

	"gorm.io/gorm"
)

// Seed заполняет базу тестовыми данными, если она пустая
func Seed(db *gorm.DB) {
	var count int64
	db.Model(&domain.CourseClass{}).Count(&count)

	if count > 0 {
		log.Println("Database already seeded. Skipping...")
		return
	}

	log.Println("Starting Smart Seeder: generating complex dataset...")
	rand.Seed(time.Now().UnixNano()) // Для случайного распределения преподавателей

	// ==========================================
	// 1. ГЕНЕРАЦИЯ СЕТКИ ВРЕМЕНИ (20 слотов)
	// ==========================================
	days := []domain.DayOfWeek{
		domain.DayMonday, domain.DayTuesday, domain.DayWednesday, domain.DayThursday, domain.DayFriday,
	}
	times := [][2]string{
		{"09:00", "10:30"}, // 1 пара
		{"10:40", "12:10"}, // 2 пара
		{"13:00", "14:30"}, // 3 пара
		{"14:40", "16:10"}, // 4 пара
	}

	var slots []domain.TimeSlot
	for _, day := range days {
		for period := 1; period <= 4; period++ {
			slots = append(slots, domain.TimeSlot{
				Day:          day,
				PeriodNumber: period,
				StartTime:    times[period-1][0],
				EndTime:      times[period-1][1],
			})
		}
	}
	db.Create(&slots)

	// ==========================================
	// 2. ГЕНЕРАЦИЯ АУДИТОРИЙ (Разные этажи и типы)
	// ==========================================
	rooms := []domain.Room{
		// 1 этаж (Большие потоковые)
		{Name: "101-Лек", Capacity: 100, Type: domain.RoomTypeLecture, Floor: 1},
		{Name: "102-Лек", Capacity: 100, Type: domain.RoomTypeLecture, Floor: 1},
		// 2 этаж (Практики)
		{Name: "201-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		{Name: "202-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		{Name: "203-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		// 3 этаж (Компьютерные классы / Лабы)
		{Name: "301-Лаб", Capacity: 15, Type: domain.RoomTypeLab, Floor: 3},
		{Name: "302-Лаб", Capacity: 15, Type: domain.RoomTypeLab, Floor: 3},
		{Name: "303-Лаб", Capacity: 15, Type: domain.RoomTypeLab, Floor: 3},
	}
	db.Create(&rooms)

	// ==========================================
	// 3. ПРЕПОДАВАТЕЛИ И ГРУППЫ
	// ==========================================
	instructors := []domain.Instructor{
		{Name: "Иванов И.И."}, {Name: "Петров П.П."}, {Name: "Сидоров С.С."},
		{Name: "Смирнов А.А."}, {Name: "Кузнецов В.В."}, {Name: "Попов Д.Д."},
		{Name: "Соколов Е.Е."}, {Name: "Лебедев Ж.Ж."}, {Name: "Козлов З.З."},
		{Name: "Новиков К.К."},
	}
	db.Create(&instructors)

	// Допустим, у нас 2 потока (1 курс и 2 курс), по 3 группы в каждом
	groups := []domain.Group{
		{Name: "CS-101", Size: 25}, {Name: "CS-102", Size: 24}, {Name: "CS-103", Size: 26}, // 1 курс (Поток 1)
		{Name: "SE-201", Size: 20}, {Name: "SE-202", Size: 22}, {Name: "SE-203", Size: 21}, // 2 курс (Поток 2)
	}
	db.Create(&groups)

	// ==========================================
	// 4. ПРЕДМЕТЫ И УЧЕБНЫЙ ПЛАН (Кредиты и Часы)
	// ==========================================
	subjects := []domain.Subject{
		{Name: "Высшая математика", Credits: 5, LectureHours: 2, PracticeHours: 4, LabHours: 0},
		{Name: "Программирование (Go)", Credits: 6, LectureHours: 2, PracticeHours: 0, LabHours: 4},
		{Name: "Базы данных", Credits: 4, LectureHours: 2, PracticeHours: 2, LabHours: 2},
		{Name: "История", Credits: 2, LectureHours: 2, PracticeHours: 2, LabHours: 0},
		{Name: "Английский язык", Credits: 3, LectureHours: 0, PracticeHours: 4, LabHours: 0},
	}
	db.Create(&subjects)

	// ==========================================
	// 5. ГЕНЕРАЦИЯ БЛОКОВ РАСПИСАНИЯ (CourseClasses)
	// ==========================================
	// Правило: 2 академических часа = 1 блок (1 пара)
	var classes []domain.CourseClass

	// Вспомогательная функция выбора случайного препода
	getRandomInst := func() uint { return instructors[rand.Intn(len(instructors))].ID }

	// Разделим группы на потоки
	stream1 := []*domain.Group{&groups[0], &groups[1], &groups[2]} // CS
	stream2 := []*domain.Group{&groups[3], &groups[4], &groups[5]} // SE
	allStreams := [][]*domain.Group{stream1, stream2}

	for _, stream := range allStreams {
		for _, sub := range subjects {

			// 1. ЛЕКЦИИ (Потоковые)
			// Если у предмета есть лекции, создаем ОДНУ карточку для всего потока
			if sub.LectureHours > 0 {
				blocksCount := sub.LectureHours / 2 // Сколько пар нужно
				instID := getRandomInst()           // Один лектор на поток

				for i := 0; i < blocksCount; i++ {
					classes = append(classes, domain.CourseClass{
						SubjectID:        sub.ID,
						InstructorID:     instID,
						Groups:           stream, // Связываем со всеми группами потока!
						IsLecture:        true,
						RequiredRoomType: domain.RoomTypeLecture,
						Duration:         1,
					})
				}
			}

			// 2. ПРАКТИКИ (Индивидуально для каждой группы)
			if sub.PracticeHours > 0 {
				blocksCount := sub.PracticeHours / 2
				for _, grp := range stream {
					instID := getRandomInst() // Практику могут вести разные преподы
					for i := 0; i < blocksCount; i++ {
						classes = append(classes, domain.CourseClass{
							SubjectID:        sub.ID,
							InstructorID:     instID,
							Groups:           []*domain.Group{grp}, // Только одна группа!
							IsLecture:        false,
							RequiredRoomType: domain.RoomTypePractice,
							Duration:         1,
						})
					}
				}
			}

			// 3. ЛАБОРАТОРНЫЕ (Индивидуально для каждой группы)
			if sub.LabHours > 0 {
				blocksCount := sub.LabHours / 2
				for _, grp := range stream {
					instID := getRandomInst()
					for i := 0; i < blocksCount; i++ {
						classes = append(classes, domain.CourseClass{
							SubjectID:        sub.ID,
							InstructorID:     instID,
							Groups:           []*domain.Group{grp},
							IsLecture:        false,
							RequiredRoomType: domain.RoomTypeLab, // Требуем комп. класс
							Duration:         1,
						})
					}
				}
			}
		}
	}

	// Массовая вставка сгенерированных занятий
	// В нашем примере их получится около 60-70 штук.
	// Это уже создаст серьезную нагрузку для алгоритма, так как слотов всего 20!
	db.Create(&classes)

	log.Printf("Smart Seeder finished! Generated %d CourseClasses to schedule.\n", len(classes))
}
