package genetic

import (
	"math"
	"scheduler/internal/domain"
)

const (
	// Hard Constraints (Жесткие штрафы - должны быть огромными)
	PenaltyRoomConflict       = -10000.0
	PenaltyInstructorConflict = -10000.0
	PenaltyGroupConflict      = -10000.0
	PenaltyCapacityOverflow   = -5000.0

	// Soft Constraints & Bonuses (Фишечки)
	PenaltyWrongRoomType = -50.0 // Например, лекция в лабе
	BonusPerfectRoomType = +20.0 // Предмет в идеальной аудитории

	// Коэффициент сглаживания сигмоиды (чтобы график не был слишком резким)
	SigmoidScale = 0.01
)

type Evaluator struct {
	RoomsMap   map[uint]*domain.Room
	SlotsMap   map[uint]*domain.TimeSlot
	ClassesMap map[uint]*domain.CourseClass
}

func NewEvaluator(rooms []domain.Room, slots []domain.TimeSlot, classes []domain.CourseClass) *Evaluator {
	rMap := make(map[uint]*domain.Room)
	sMap := make(map[uint]*domain.TimeSlot)
	cMap := make(map[uint]*domain.CourseClass)

	for i := range rooms {
		rMap[rooms[i].ID] = &rooms[i]
	}
	for i := range slots {
		sMap[slots[i].ID] = &slots[i]
	}
	for i := range classes {
		cMap[classes[i].ID] = &classes[i]
	}

	return &Evaluator{RoomsMap: rMap, SlotsMap: sMap, ClassesMap: cMap}
}

// CalculateFitness вычисляет приспособленность через Сигмоиду
func (e *Evaluator) CalculateFitness(ind *Individual) float64 {
	hardConflicts := 0 // Счетчик жестких накладок
	softScore := 0.0   // Баллы за "фишечки"

	roomUsage := make(map[struct{ S, R uint }]bool)
	instructorUsage := make(map[struct{ S, I uint }]bool)
	groupUsage := make(map[struct{ S, G uint }]bool)

	for _, gene := range ind.Genes {
		room := e.RoomsMap[gene.RoomID]
		cls := e.ClassesMap[gene.ClassID]

		// ==========================================
		// 1. HARD CONSTRAINTS (Жесткие ограничения)
		// ==========================================
		if room.Capacity < gene.StudentsCount {
			hardConflicts++
		}

		roomKey := struct{ S, R uint }{gene.SlotID, gene.RoomID}
		if roomUsage[roomKey] {
			hardConflicts++
		}
		roomUsage[roomKey] = true

		instKey := struct{ S, I uint }{gene.SlotID, gene.InstructorID}
		if instructorUsage[instKey] {
			hardConflicts++
		}
		instructorUsage[instKey] = true

		for _, gid := range gene.GroupIDs {
			grpKey := struct{ S, G uint }{gene.SlotID, gid}
			if groupUsage[grpKey] {
				hardConflicts++
			}
			groupUsage[grpKey] = true
		}

		// ==========================================
		// 2. SOFT CONSTRAINTS (Бонусы и мелкие штрафы)
		// ==========================================
		if room.Type == cls.RequiredRoomType {
			softScore += 1.0 // Бонус за правильный тип аудитории
		} else {
			softScore -= 1.0 // Штраф за дискомфорт
		}
	}

	// ==========================================
	// 3. МАТЕМАТИКА ФИТНЕСА
	// ==========================================

	// Базовый фитнес за отсутствие коллизий (от 0.0 до 1.0)
	// Если hardConflicts = 0, baseFitness = 1.0
	// Если hardConflicts = 1, baseFitness = 0.5
	// Если hardConflicts = 9, baseFitness = 0.1
	baseFitness := 1.0 / (1.0 + float64(hardConflicts))

	// Сигмоида для мягких бонусов (от 0.0 до 1.0)
	// softScore может быть отрицательным или положительным
	softSigmoid := 1.0 / (1.0 + math.Exp(-0.1*softScore))

	// Объединяем. Вес бонусов делаем маленьким (например, 0.1),
	// чтобы они работали как "усилитель вкуса", но не могли перебить коллизию.
	// Максимальный фитнес теперь может быть 1.1!
	totalFitness := baseFitness + (softSigmoid * 0.1)

	// Сохраняем в объект (чтобы мы могли выводить это в логи)
	ind.Fitness = totalFitness

	return totalFitness
}
