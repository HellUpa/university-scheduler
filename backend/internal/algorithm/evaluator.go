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

func (e *Evaluator) CountConflicts(schedule *Schedule) (hardConflicts int, softPenalties, softBonuses float64) {
	// 1. Предварительная сборка данных (чтобы правилам было проще)
	e.buildGroupDailySchedule(schedule)

	hardConflicts = 0
	softPenalties = 0.0
	softBonuses = 0.0

	// 2. Прогоняем расписание через ВСЕ правила
	for _, rule := range e.Rules {
		h, sp, sb := rule(schedule, e.Data)
		hardConflicts += h
		softPenalties += sp
		softBonuses += sb
	}

	return hardConflicts, softPenalties, softBonuses
}

// CalculateFitness теперь просто оркестратор
func (e *Evaluator) CalculateFitness(schedule *Schedule) float64 {
	hardConflicts, softPenalties, softBonuses := e.CountConflicts(schedule)

	// Масштабируем
	scaleFactor := float64(len(schedule.Assignments))
	relPenalties := float64(softPenalties) / scaleFactor
	relBonuses := float64(softBonuses) / scaleFactor

	// ==========================================
	// 1. СЧИТАЕМ ОЦЕНКУ МЯГКИХ ОГРАНИЧЕНИЙ (СТРОГО ОТ 0.0 ДО 1.0)
	// ==========================================

	// Коэффициенты чувствительности (можно вынести в конфиг)
	// Они определяют, насколько быстро падает/растет кривая
	penaltySensitivity := e.Data.Config.PenaltyScale
	bonusSensitivity := e.Data.Config.BonusScale

	// Оценка по штрафам (от 1.0 до 0.0)
	// 0 штрафов = 1.0. Много штрафов -> 0.0
	penaltyScore := math.Exp(relPenalties * penaltySensitivity)

	// Оценка по бонусам (от 0.0 до 1.0)
	// 0 бонусов = 0.0. Много бонусов -> стремится к 1.0
	b := relBonuses * bonusSensitivity
	bonusScore := b / (1.0 + b)

	// Взвешиваем их. Сумма весов ДОЛЖНА БЫТЬ = 1.0
	// Так как ты предпочитаешь штрафовать, даем штрафам больший вес.
	weightPenalty := 0.7 // 70% успеха мягких ограничений - это отсутствие штрафов
	weightBonus := 0.3   // 30% успеха - это наличие бонусов

	dynamicBonusWeight := weightBonus * penaltyScore

	// Собираем итог.
	normalizedSoftScore := (penaltyScore * weightPenalty) + (bonusScore * dynamicBonusWeight)

	// ==========================================
	// 2. ФОРМИРУЕМ ИТОГОВЫЙ ФИТНЕС
	// ==========================================

	var totalFitness float64

	if hardConflicts == 0 {
		// ВАЛИДНОЕ РЕШЕНИЕ
		// База всегда 1.0 + оценка мягких ограничений (от 0 до 1)
		// Диапазон: [1.0, 2.0)

		// Худшее расписание (много штрафов, 0 бонусов): 1.0 + 0.0 = 1.0
		// Идеальное расписание (0 штрафов, бесконечно бонусов): 1.0 + 1.0 = 2.0
		totalFitness = 1.0 + normalizedSoftScore
	} else {
		// НЕВАЛИДНОЕ РЕШЕНИЕ
		// База = 1 / (1 + Hard) => 0.5, 0.33, 0.25...
		baseFitness := 1.0 / (1.0 + float64(hardConflicts))

		// Чтобы дать алгоритму понимать, что он на правильном пути,
		// добавляем к базе микро-множитель на основе мягких параметров.
		// Диапазон множителя: [1.0, 1.5).
		// При Hard=1 максимум будет 0.5 * 1.5 = 0.75 (строго меньше 1.0!)
		softMultiplier := 1.0 + (normalizedSoftScore * 0.5)

		totalFitness = baseFitness * softMultiplier
	}

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
