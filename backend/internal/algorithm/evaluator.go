package algorithm

import (
	"fmt"
	"math"
	"scheduler/internal/domain"
)

type Evaluator struct {
	Data  *EvalData
	Rules []Rule
}

func NewEvaluator(rooms []domain.Room, slots []domain.TimeSlot, classes []domain.CourseClass) *Evaluator {
	data := &EvalData{
		Config:     DefaultConfig,
		RoomsMap:   make(map[uint]*domain.Room),
		SlotsMap:   make(map[uint]*domain.TimeSlot),
		ClassesMap: make(map[uint]*domain.CourseClass),
	}

	for i := range rooms {
		data.RoomsMap[rooms[i].ID] = &rooms[i]
	}
	for i := range slots {
		data.SlotsMap[slots[i].ID] = &slots[i]
	}
	for i := range classes {
		data.ClassesMap[classes[i].ID] = &classes[i]
	}

	rules := []Rule{
		// Жесткие
		RuleCapacity,
		RuleUnassigned,
		RuleOverlaps,
		// Мягкие
		RuleRoomType,
		RuleGaps,
		RuleCompactness,
		RuleOverloadedDay,
		RuleLectureBeforePractice,
	}

	return &Evaluator{
		Data:  data,
		Rules: rules,
	}
}

func (e *Evaluator) CountConflicts(schedule *Schedule) (hardConflicts int, softConflicts float64) {
	// 1. Предварительная сборка данных (чтобы правилам было проще)
	e.buildGroupDailySchedule(schedule)

	hardConflicts = 0
	softConflicts = 0.0

	// 2. Прогоняем расписание через ВСЕ правила
	for _, rule := range e.Rules {
		h, s := rule(schedule, e.Data)
		hardConflicts += h
		softConflicts += s
	}

	return hardConflicts, softConflicts
}

// CalculateFitness теперь просто оркестратор
func (e *Evaluator) CalculateFitness(schedule *Schedule) float64 {
	totalHardConflicts, totalSoftScore := e.CountConflicts(schedule)

	// Делаем поправку на масштаб
	scaleFactor := float64(len(schedule.Assignments))
	relativeSoftScore := totalSoftScore / scaleFactor

	// 1. Математика
	baseFitness := 1.0 / (1.0 + float64(totalHardConflicts))

	// 2. Считаем бонусы с помощью Softsign (от 0 до 1)
	x := e.Data.Config.FuncScaleFactor * relativeSoftScore
	// Softsign функция: x / (1 + |x|)
	softsign := x / (1.0 + math.Abs(x))

	// Нормализуем из [-1, 1] в [0, 1]
	softScoreNormalized := (softsign + 1.0) / 2.0

	// 3. Итоговый фитнес
	// Теперь вес бонусов - это вес нормализованного значения
	totalFitness := baseFitness * (1.0 + (softScoreNormalized * e.Data.Config.SoftScoreWeight))

	schedule.Fitness = totalFitness
	return totalFitness
}

// Вспомогательная функция для сборки расписания по дням
func (e *Evaluator) buildGroupDailySchedule(schedule *Schedule) {
	// Очищаем/инициализируем мапу для текущей хромосомы
	schedule.GroupDailySchedule = make(map[uint]map[domain.DayOfWeek][]int)

	for _, assignment := range schedule.Assignments {
		slot := e.Data.SlotsMap[assignment.SlotID]
		for _, gid := range assignment.GroupIDs {
			if schedule.GroupDailySchedule[gid] == nil {
				schedule.GroupDailySchedule[gid] = make(map[domain.DayOfWeek][]int)
			}
			schedule.GroupDailySchedule[gid][slot.Day] = append(schedule.GroupDailySchedule[gid][slot.Day], slot.PeriodNumber)
		}
	}
}

// DebugConflicts выводит в консоль расшифровку всех жестких конфликтов
func DebugConflicts(schedule *Schedule, data *EvalData) {
	fmt.Println("=== ДЕБАГ ЖЕСТКИХ КОНФЛИКТОВ ===")
	roomUsage := make(map[struct{ S, R uint }]string)
	instructorUsage := make(map[struct{ S, I uint }]string)
	groupUsage := make(map[struct{ S, G uint }]string)

	unassigned := 0

	for _, assignment := range schedule.Assignments {
		if assignment.SlotID == 0 || assignment.RoomID == 0 {
			unassigned++
			continue
		}

		cls := data.ClassesMap[assignment.ClassID]
		room := data.RoomsMap[assignment.RoomID]
		slot := data.SlotsMap[assignment.SlotID]
		dayTime := fmt.Sprintf("%s %s", slot.Day, slot.StartTime)

		// 1. Вместимость
		if room.Capacity < assignment.StudentsCount {
			fmt.Printf("[ВМЕСТИМОСТЬ] Предмет '%s' (%d чел) не лезет в ауд. %s (%d мест)\n",
				cls.Subject.Name, assignment.StudentsCount, room.Name, room.Capacity)
		}

		// 2. Аудитории
		roomKey := struct{ S, R uint }{assignment.SlotID, assignment.RoomID}
		if prev, exists := roomUsage[roomKey]; exists {
			fmt.Printf("[АУДИТОРИЯ] Накладка в %s, ауд. %s: '%s' пересекается с '%s'\n",
				dayTime, room.Name, prev, cls.Subject.Name)
		}
		roomUsage[roomKey] = cls.Subject.Name

		// 3. Преподы
		instKey := struct{ S, I uint }{assignment.SlotID, cls.InstructorID}
		if prev, exists := instructorUsage[instKey]; exists {
			fmt.Printf("[ПРЕПОДАВАТЕЛЬ] %s в %s ведет '%s' и '%s' одновременно!\n",
				cls.Instructor.Name, dayTime, prev, cls.Subject.Name)
		}
		instructorUsage[instKey] = cls.Subject.Name

		// 4. Группы
		for _, group := range cls.Groups {
			grpKey := struct{ S, G uint }{assignment.SlotID, group.ID}
			if prev, exists := groupUsage[grpKey]; exists {
				fmt.Printf("[ГРУППА] Группа %s в %s должна быть на '%s' и на '%s'\n",
					group.Name, dayTime, prev, cls.Subject.Name)
			}
			groupUsage[grpKey] = cls.Subject.Name
		}
	}

	if unassigned > 0 {
		fmt.Printf("[ПРОПУСКИ] Не распределено занятий: %d\n", unassigned)
	}
	fmt.Println("=================================")
}
