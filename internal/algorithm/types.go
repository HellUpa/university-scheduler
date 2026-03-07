package algorithm

import "scheduler/internal/domain"

// Assignment - универсальное "Назначение"
type Assignment struct {
	ClassID uint
	RoomID  uint
	SlotID  uint

	// Кэшированные поля для быстрого расчета (чтобы не лазить в БД в цикле)
	InstructorID  uint
	GroupIDs      []uint
	StudentsCount int
}

// Schedule - вариант полного расписания
type Schedule struct {
	Assignments []*Assignment
	Fitness     float64

	GroupDailySchedule map[uint]map[domain.DayOfWeek][]int
}

// Конструктор
func NewSchedule(assignments []*Assignment) *Schedule {
	return &Schedule{
		Assignments:        assignments,
		Fitness:            0.0,
		GroupDailySchedule: make(map[uint]map[domain.DayOfWeek][]int),
	}
}
