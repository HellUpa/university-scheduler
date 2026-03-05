package genetic

import (
	"math"
	"scheduler/internal/domain"
	"sort"
)

const (
	// SOFT CONSTRAINTS & BONUSES (Перебалансировано)
	// Штрафы за дискомфорт
	PenaltyGap           = -20.0 // Штраф за окно сделали меньше
	PenaltyWrongRoomType = -10.0 // Неправильный тип аудитории

	// Бонусы за идеальные условия
	BonusPerfectRoomType = +5.0  // Бонус за аудиторию тоже стал меньше
	BonusDayWithoutGaps  = +50.0 // БОНУС за идеальный день без окон!

	// Коэффициенты для итогового фитнеса
	SigmoidScaleFactor = 0.01 // Коэффициент сглаживания сигмоиды
	SoftScoreWeight    = 0.5  // Вес "бонусной" части в итоговом фитнесе
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
	hardConflicts := 0
	softScore := 0.0

	// Карты коллизий (как раньше)
	roomUsage := make(map[struct{ S, R uint }]bool)
	instructorUsage := make(map[struct{ S, I uint }]bool)
	groupUsage := make(map[struct{ S, G uint }]bool)

	// Map[GroupID][DayOfWeek] -> Slice of PeriodNumbers
	groupDailySchedule := make(map[uint]map[domain.DayOfWeek][]int)

	for _, gene := range ind.Genes {
		room := e.RoomsMap[gene.RoomID]
		slot := e.SlotsMap[gene.SlotID]
		cls := e.ClassesMap[gene.ClassID]

		// --- 1. HARD CHECKS---
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

			// --- СБОР ДАННЫХ ДЛЯ ОКОН ---
			// Инициализируем мапу, если нет
			if groupDailySchedule[gid] == nil {
				groupDailySchedule[gid] = make(map[domain.DayOfWeek][]int)
			}
			// Записываем, что у группы gid в этот день занят слот period
			groupDailySchedule[gid][slot.Day] = append(groupDailySchedule[gid][slot.Day], slot.PeriodNumber)
		}

		// --- 2. SOFT CHECKS (Тип аудитории) ---
		if room.Type == cls.RequiredRoomType {
			softScore += BonusPerfectRoomType
		} else {
			softScore += PenaltyWrongRoomType
		}
	}

	// --- 3. АНАЛИЗ ОКОН (GAPS ANALYSIS) ---
	for _, daysMap := range groupDailySchedule {
		for _, periods := range daysMap {
			if len(periods) < 2 {
				continue
			}
			sort.Ints(periods)

			hasGaps := false
			for i := 1; i < len(periods); i++ {
				diff := periods[i] - periods[i-1]
				if diff > 1 {
					gapsCount := diff - 1
					softScore += float64(gapsCount) * PenaltyGap
					hasGaps = true
				}
			}

			// !!! Если в этом дне НЕ было окон, даем СУПЕР-БОНУС !!!
			if !hasGaps {
				softScore += BonusDayWithoutGaps
			}
		}
	}

	// --- 4. ИТОГОВЫЙ РАСЧЕТ ---
	baseFitness := 1.0 / (1.0 + float64(hardConflicts))

	// Если есть жесткие конфликты, бонусы нас не интересуют.
	// Это помогает алгоритму на ранних стадиях сосредоточиться на главном.
	if baseFitness < 1.0 {
		ind.Fitness = baseFitness
		return baseFitness
	}

	softSigmoid := 1.0 / (1.0 + math.Exp(-SigmoidScaleFactor*softScore))

	// Итоговый фитнес = 1.0 (за отсутствие коллизий) + до 0.5 очков за бонусы
	totalFitness := baseFitness + (softSigmoid * SoftScoreWeight)

	ind.Fitness = totalFitness
	return totalFitness
}
