package algorithm

import (
	"math"
	"scheduler/internal/domain"
)

type Evaluator struct {
	Context *EvalContext
	Rules   []Rule
}

func NewEvaluator(rooms []domain.Room, slots []domain.TimeSlot, classes []domain.CourseClass) *Evaluator {
	ctx := &EvalContext{
		Config:     DefaultConfig,
		RoomsMap:   make(map[uint]*domain.Room),
		SlotsMap:   make(map[uint]*domain.TimeSlot),
		ClassesMap: make(map[uint]*domain.CourseClass),
	}

	for i := range rooms {
		ctx.RoomsMap[rooms[i].ID] = &rooms[i]
	}
	for i := range slots {
		ctx.SlotsMap[slots[i].ID] = &slots[i]
	}
	for i := range classes {
		ctx.ClassesMap[classes[i].ID] = &classes[i]
	}

	rules := []Rule{
		RuleCapacity, // Жесткие
		RuleOverlaps, // Жесткие
		RuleRoomType, // Мягкие
		RuleGaps,     // Мягкие
	}

	return &Evaluator{
		Context: ctx,
		Rules:   rules,
	}
}

// CalculateFitness теперь просто оркестратор
func (e *Evaluator) CalculateFitness(schedule *Schedule) float64 {
	// 1. Предварительная сборка данных (чтобы правилам было проще)
	e.buildGroupDailySchedule(schedule)

	totalHardConflicts := 0
	totalSoftScore := 0.0

	// 2. Прогоняем расписание через ВСЕ правила
	for _, rule := range e.Rules {
		h, s := rule(schedule, e.Context)
		totalHardConflicts += h
		totalSoftScore += s
	}

	// 3. Математика
	baseFitness := 1.0 / (1.0 + float64(totalHardConflicts))

	if baseFitness < 1.0 {
		schedule.Fitness = baseFitness
		return baseFitness
	}

	// Считаем бонусы только если расписание валидно (baseFitness == 1.0)
	softSigmoid := 1.0 / (1.0 + math.Exp(-e.Context.Config.SigmoidScaleFactor*totalSoftScore))
	totalFitness := baseFitness + (softSigmoid * e.Context.Config.SoftScoreWeight)

	schedule.Fitness = totalFitness
	return totalFitness
}

// Вспомогательная функция для сборки расписания по дням
func (e *Evaluator) buildGroupDailySchedule(schedule *Schedule) {
	// Очищаем/инициализируем мапу для текущей хромосомы
	schedule.GroupDailySchedule = make(map[uint]map[domain.DayOfWeek][]int)

	for _, assignment := range schedule.Assignments {
		slot := e.Context.SlotsMap[assignment.SlotID]
		for _, gid := range assignment.GroupIDs {
			if schedule.GroupDailySchedule[gid] == nil {
				schedule.GroupDailySchedule[gid] = make(map[domain.DayOfWeek][]int)
			}
			schedule.GroupDailySchedule[gid][slot.Day] = append(schedule.GroupDailySchedule[gid][slot.Day], slot.PeriodNumber)
		}
	}
}
