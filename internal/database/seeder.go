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

	// Сетка: 50 мин урок + 10 мин перемена (кроме больших по 20 мин)
	times := [][2]string{
		{"08:00", "08:50"}, // 1 слот
		{"09:00", "09:50"}, // 2 слот
		{"10:00", "10:50"}, // 3 слот
		// --- Большая перемена 20 мин ---
		{"11:10", "12:00"}, // 4 слот
		{"12:10", "13:00"}, // 5 слот
		{"13:10", "14:00"}, // 6 слот
		{"14:10", "15:00"}, // 7 слот
		{"15:10", "16:00"}, // 8 слот
		// --- Большая перемена 20 мин ---
		{"16:20", "17:10"}, // 9 слот
		{"17:20", "18:10"}, // 10 слот
	}

	var slots []domain.TimeSlot
	for _, day := range days {
		for i, time := range times {
			slots = append(slots, domain.TimeSlot{
				Day:          day,
				PeriodNumber: i + 1,
				StartTime:    time[0],
				EndTime:      time[1],
			})
		}
	}
	db.Create(&slots)

	// ==========================================
	// 2. ГЕНЕРАЦИЯ АУДИТОРИЙ (Разные этажи и типы)
	// ==========================================
	rooms := []domain.Room{
		// 1 этаж (Лекции)
		{Name: "101-Лек", Capacity: 100, Type: domain.RoomTypeLecture, Floor: 1},
		{Name: "102-Лек", Capacity: 100, Type: domain.RoomTypeLecture, Floor: 1},
		{Name: "103-Лек", Capacity: 100, Type: domain.RoomTypeLecture, Floor: 1},
		{Name: "104-Лек", Capacity: 100, Type: domain.RoomTypeLecture, Floor: 1},
		// 2 этаж (Практики)
		{Name: "201-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		{Name: "202-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		{Name: "203-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		{Name: "204-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		{Name: "205-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		{Name: "206-Пр", Capacity: 30, Type: domain.RoomTypePractice, Floor: 2},
		// 3 этаж (Лабы)
		{Name: "301-Лаб", Capacity: 30, Type: domain.RoomTypeLab, Floor: 3},
		{Name: "302-Лаб", Capacity: 30, Type: domain.RoomTypeLab, Floor: 3},
		{Name: "303-Лаб", Capacity: 30, Type: domain.RoomTypeLab, Floor: 3},
		{Name: "304-Лаб", Capacity: 30, Type: domain.RoomTypeLab, Floor: 3},
		{Name: "305-Лаб", Capacity: 30, Type: domain.RoomTypeLab, Floor: 3},
		{Name: "306-Лаб", Capacity: 30, Type: domain.RoomTypeLab, Floor: 3},
	}
	db.Create(&rooms)

	// ==========================================
	// 3. ПРЕПОДАВАТЕЛИ И ГРУППЫ
	// ==========================================
	instructors := []domain.Instructor{
		{Name: "Иванов И.И."}, {Name: "Петров П.П."}, {Name: "Сидоров С.С."},
		{Name: "Смирнов А.А."}, {Name: "Кузнецов В.В."}, {Name: "Попов Д.Д."},
		{Name: "Соколов Е.Е."}, {Name: "Лебедев Ж.Ж."}, {Name: "Козлов З.З."},
		{Name: "Новиков К.К."}, {Name: "Морозов М.М."}, {Name: "Волков В.В."},
		{Name: "Алексеев А.А."}, {Name: "Николаев Н.Н."}, {Name: "Макаров М.М."},
		{Name: "Алексеева В.Г."}, {Name: "Радионов Н.Н."}, {Name: "Лукашенко М.М."},
		// {Name: "Гудко А.А."}, {Name: "Жаринов Н.Н."}, {Name: "Кириченко М.М."},
		// {Name: "Андропов В.Г."}, {Name: "Михалков Н.Н."}, {Name: "Кижуч М.М."},
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
	// Формула: 5 кредитов = 1 час лекций + 2 часа практик/лаб.
	subjects := []domain.Subject{
		{Name: "Высшая математика", Credits: 6, LectureHours: 2, PracticeHours: 2, LabHours: 0}, // 4 ч
		{Name: "Физика", Credits: 4, LectureHours: 1, PracticeHours: 1, LabHours: 1},            // 3 ч

		// Профиль
		{Name: "Программирование (Go)", Credits: 8, LectureHours: 2, PracticeHours: 0, LabHours: 4}, // 6 ч
		{Name: "Алгоритмы и структуры", Credits: 5, LectureHours: 1, PracticeHours: 0, LabHours: 2}, // 3 ч

		// Гуманитарные
		{Name: "История", Credits: 3, LectureHours: 1, PracticeHours: 1, LabHours: 0},         // 2 ч
		{Name: "Английский язык", Credits: 4, LectureHours: 0, PracticeHours: 4, LabHours: 0}, // 4 ч
	}
	db.Create(&subjects)

	// ==========================================
	// 5. ГЕНЕРАЦИЯ БЛОКОВ РАСПИСАНИЯ (CourseClasses)
	// ==========================================
	// Правило: 2 академических часа = 1 блок (1 пара)
	var classes []domain.CourseClass

	instIndex := 0
	getNextInst := func() uint {
		id := instructors[instIndex].ID
		instIndex = (instIndex + 1) % len(instructors)
		return id
	}

	// Разделим группы на потоки
	stream1 := []*domain.Group{&groups[0], &groups[1], &groups[2]} // CS
	stream2 := []*domain.Group{&groups[3], &groups[4], &groups[5]} // SE
	allStreams := [][]*domain.Group{stream1, stream2}

	for _, stream := range allStreams {
		for _, sub := range subjects {

			// 1. ЛЕКЦИИ (Потоковые)
			// Если у предмета есть лекции, создаем ОДНУ карточку для всего потока
			if sub.LectureHours > 0 {
				blocksCount := sub.LectureHours // Сколько пар нужно
				instID := getNextInst()         // Один лектор на поток

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
				blocksCount := sub.PracticeHours
				for _, grp := range stream {
					instID := getNextInst() // Практику могут вести разные преподы
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
				blocksCount := sub.LabHours
				for _, grp := range stream {
					instID := getNextInst()
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
