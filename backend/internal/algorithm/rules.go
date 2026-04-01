package algorithm

import (
	"scheduler/internal/domain"
	"sort"
)

// EvaluatorConfig хранит веса и штрафы.
// В будущем это можно принимать прямо из JSON запроса!
type EvaluatorConfig struct {
	// Настройки мягких ограничений
	MaxClassesPerDay int

	// Штрафы за мягкие ограничения
	PenaltyGap                  float64
	PenaltyWrongRoomType        float64
	BonusPerfectRoomType        float64
	BonusDayWithoutGaps         float64
	PenaltyOverloadedDay        float64
	PenaltyLectureAfterPractice float64

	// настройки расчета для функции мягких ограничений
	TanhScaleFactor float64
	SoftScoreWeight float64
}

// DefaultConfig - настройки по умолчанию
var DefaultConfig = EvaluatorConfig{
	PenaltyGap:           -20.0,
	PenaltyWrongRoomType: -10.0,

	BonusPerfectRoomType: +5.0,
	BonusDayWithoutGaps:  +20.0,

	MaxClassesPerDay:     5,
	PenaltyOverloadedDay: -100.0,

	PenaltyLectureAfterPractice: -25.0,

	TanhScaleFactor: 0.3,
	SoftScoreWeight: 0.5,
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

// RuleCompactness награждает за то, что занятия стоят в начале дня
func RuleCompactness(schedule *Schedule, ctx *EvalContext) (int, float64) {
	bonus := 0.0
	for _, assignment := range schedule.Assignments {
		if assignment.SlotID == 0 {
			continue
		}

		slot := ctx.SlotsMap[assignment.SlotID]
		// Чем меньше номер периода (1, 2, 3...), тем больше бонус
		// Например: 10 - номер периода. Пара в 08:00 (1) даст +9 баллов.
		bonus += float64(11-slot.PeriodNumber) * 0.5
	}
	return 0, bonus
}

// RuleOverloadedDay штрафует за слишком большое количество занятий у группы в один день.
func RuleOverloadedDay(schedule *Schedule, ctx *EvalContext) (int, float64) {
	softScore := 0.0

	// Проходим по расписанию каждой группы
	for _, dailySchedule := range schedule.GroupDailySchedule {
		// Проходим по дням недели для этой группы
		for _, slotsInDay := range dailySchedule {
			numClasses := len(slotsInDay)

			// Если количество пар в день превышает "комфортное"
			if numClasses > ctx.Config.MaxClassesPerDay {
				// Штраф может быть прогрессивным.
				// За 5-ю пару - один штраф, за 6-ю - уже два, и т.д.
				overload := numClasses - ctx.Config.MaxClassesPerDay
				penalty := float64(overload) * ctx.Config.PenaltyOverloadedDay
				softScore += penalty
			}
		}
	}

	return 0, softScore
}

// RuleLectureBeforePractice штрафует, если практическое занятие по предмету
// стоит в расписании раньше лекции.
func RuleLectureBeforePractice(schedule *Schedule, ctx *EvalContext) (int, float64) {
	softScore := 0.0

	// 1. Группируем все занятия по ID предмета
	assignmentsBySubject := make(map[uint][]*Assignment)
	for _, assign := range schedule.Assignments {
		classInfo := ctx.ClassesMap[assign.ClassID]
		subjectID := classInfo.SubjectID
		assignmentsBySubject[subjectID] = append(assignmentsBySubject[subjectID], assign)
	}

	// 2. Анализируем каждую группу предметов
	for _, assignments := range assignmentsBySubject {
		var lectureSlots []uint
		var practiceSlots []uint

		// Разделяем на лекции и практики
		for _, assign := range assignments {
			classInfo := ctx.ClassesMap[assign.ClassID]
			if classInfo.IsLecture {
				lectureSlots = append(lectureSlots, assign.SlotID)
			} else {
				practiceSlots = append(practiceSlots, assign.SlotID)
			}
		}

		// Если есть и лекции, и практики по этому предмету
		if len(lectureSlots) > 0 && len(practiceSlots) > 0 {
			// Находим самый ранний слот лекции
			minLectureSlot := uint(9999) // Большое число
			for _, slot := range lectureSlots {
				if slot < minLectureSlot {
					minLectureSlot = slot
				}
			}

			// Проверяем каждую практику
			for _, pSlot := range practiceSlots {
				if pSlot < minLectureSlot {
					softScore += ctx.Config.PenaltyLectureAfterPractice
				}
			}
		}
	}

	return 0, softScore
}
