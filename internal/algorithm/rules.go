package algorithm

import (
	"scheduler/internal/domain"
	"sort"
)

// EvaluatorConfig хранит веса и штрафы.
// В будущем это можно принимать прямо из JSON запроса!
type EvaluatorConfig struct {
	PenaltyGap           float64
	PenaltyWrongRoomType float64
	BonusPerfectRoomType float64
	BonusDayWithoutGaps  float64
	SigmoidScaleFactor   float64
	SoftScoreWeight      float64
}

// DefaultConfig - настройки по умолчанию
var DefaultConfig = EvaluatorConfig{
	PenaltyGap:           -20.0,
	PenaltyWrongRoomType: -10.0,
	BonusPerfectRoomType: +5.0,
	BonusDayWithoutGaps:  +50.0,
	SigmoidScaleFactor:   0.01,
	SoftScoreWeight:      0.5,
}

// EvalContext содержит все предзагруженные данные для быстрого доступа
type EvalContext struct {
	Config     EvaluatorConfig
	RoomsMap   map[uint]*domain.Room
	SlotsMap   map[uint]*domain.TimeSlot
	ClassesMap map[uint]*domain.CourseClass
}

// Rule - сигнатура функции-правила
// Возвращает количество жестких конфликтов (hardConflicts) и баллы комфорта (softScore)
type Rule func(schedule *Schedule, ctx *EvalContext) (int, float64)

// ==========================================
// РЕАЛИЗАЦИЯ ПРАВИЛ (RULES)
// ==========================================

// RuleCapacity проверяет вместимость аудиторий
func RuleCapacity(schedule *Schedule, ctx *EvalContext) (int, float64) {
	hardConflicts := 0
	for _, assignment := range schedule.Assignments {
		room := ctx.RoomsMap[assignment.RoomID]
		if room.Capacity < assignment.StudentsCount {
			hardConflicts++
		}
	}
	return hardConflicts, 0.0
}

func RuleUnassigned(schedule *Schedule, ctx *EvalContext) (int, float64) {
	assignedCount := 0
	for _, assignment := range schedule.Assignments {
		if assignment.SlotID != 0 || assignment.RoomID == 0 { // Считаем 0 как "не назначено"
			assignedCount++
		}
	}
	unassignedCount := len(schedule.Assignments) - assignedCount

	return unassignedCount, 0.0
}

// RuleOverlaps проверяет накладки: одна аудитория/препод/группа в одно время
func RuleOverlaps(schedule *Schedule, ctx *EvalContext) (int, float64) {
	hardConflicts := 0
	roomUsage := make(map[struct{ S, R uint }]bool)
	instructorUsage := make(map[struct{ S, I uint }]bool)
	groupUsage := make(map[struct{ S, G uint }]bool)

	for _, assignment := range schedule.Assignments {
		// 1. Получаем ИСТИННЫЕ данные о классе
		cls := ctx.ClassesMap[assignment.ClassID]

		// Проверка аудитории
		roomKey := struct{ S, R uint }{assignment.SlotID, assignment.RoomID}
		if roomUsage[roomKey] {
			hardConflicts++
		}
		roomUsage[roomKey] = true

		// Проверка преподавателя (берем ID из cls, а не из assignment, для надежности)
		instKey := struct{ S, I uint }{assignment.SlotID, cls.InstructorID}
		if instructorUsage[instKey] {
			hardConflicts++
		}
		instructorUsage[instKey] = true

		// Проверка групп (Берем из CLS, а не из assignment!)
		for _, group := range cls.Groups {
			gid := group.ID // Получаем реальный ID

			grpKey := struct{ S, G uint }{assignment.SlotID, gid}
			if groupUsage[grpKey] {
				hardConflicts++
			}
			groupUsage[grpKey] = true
		}
	}
	return hardConflicts, 0.0
}

// RuleRoomType проверяет соответствие типа аудитории (Лекция/Лаба)
func RuleRoomType(schedule *Schedule, ctx *EvalContext) (int, float64) {
	softScore := 0.0
	for _, assignment := range schedule.Assignments {
		room := ctx.RoomsMap[assignment.RoomID]
		cls := ctx.ClassesMap[assignment.ClassID]

		if room.Type == cls.RequiredRoomType {
			softScore += ctx.Config.BonusPerfectRoomType
		} else {
			softScore += ctx.Config.PenaltyWrongRoomType
		}
	}
	return 0, softScore
}

// RuleGaps анализирует "окна" в расписании студентов
func RuleGaps(schedule *Schedule, ctx *EvalContext) (int, float64) {
	softScore := 0.0

	for _, daysMap := range schedule.GroupDailySchedule {
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
					softScore += float64(gapsCount) * ctx.Config.PenaltyGap
					hasGaps = true
				}
			}

			// Если день загружен, но окон нет - даем супер-бонус
			if !hasGaps {
				softScore += ctx.Config.BonusDayWithoutGaps
			}
		}
	}
	return 0, softScore
}
