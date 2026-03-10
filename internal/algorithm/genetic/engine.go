package genetic

import (
	"log"
	"math/rand"
	"scheduler/internal/algorithm"
	"scheduler/internal/domain"
	"sort"
	"sync"

	"gorm.io/gorm"
)

// MutationFunc - это тип для наших функций мутации (жесткой и мягкой)
type MutationFunc func(*algorithm.Schedule, float64)

type GeneticEngine struct {
	DB             *gorm.DB
	PopulationSize int
	Generations    int
	MutationRate   float64

	// Контекст
	Classes []domain.CourseClass
	RoomIDs []uint
	SlotIDs []uint

	// Кэши для быстрого поиска O(1) по ID
	ClassesMap map[uint]*domain.CourseClass
	Evaluator  *algorithm.Evaluator
}

func NewEngine(db *gorm.DB) *GeneticEngine {
	return &GeneticEngine{
		DB:             db,
		PopulationSize: 100,
		Generations:    100,
		MutationRate:   0.01,
	}
}

func (eng *GeneticEngine) Prepare() error {
	// ВАЖНО: Добавили Preload для Subject и Instructor
	err := eng.DB.
		Preload("Groups").
		Preload("Subject").
		Preload("Instructor").
		Find(&eng.Classes).Error
	if err != nil {
		return err
	}

	// Инициализируем наш кэш классов
	eng.ClassesMap = make(map[uint]*domain.CourseClass)
	for i := range eng.Classes {
		eng.ClassesMap[eng.Classes[i].ID] = &eng.Classes[i]
	}

	var rooms []domain.Room
	if err := eng.DB.Find(&rooms).Error; err != nil {
		return err
	}

	var slots []domain.TimeSlot
	if err := eng.DB.Find(&slots).Error; err != nil {
		return err
	}

	// TODO: Добавить кастомную конфигурацию
	eng.Evaluator = algorithm.NewEvaluator(nil, rooms, slots, eng.Classes)

	for _, r := range rooms {
		eng.RoomIDs = append(eng.RoomIDs, r.ID)
	}
	for _, s := range slots {
		eng.SlotIDs = append(eng.SlotIDs, s.ID)
	}

	return nil
}

func (eng *GeneticEngine) Run() (*algorithm.Schedule, error) {
	if err := eng.Prepare(); err != nil {
		return nil, err
	}

	population := make([]*algorithm.Schedule, eng.PopulationSize)

	for i := 0; i < eng.PopulationSize; i++ {
		population[i] = eng.createRandomIndividual()
	}

	bestFitnessOverall := 0.0
	stagnantGenerations := 0

	// Переменные для импульсной мутации
	shockMode := false   // Флаг "Режим удара"
	recoveryCounter := 0 // Счетчик поколений восстановления (иммунитет)
	// =========================================

	for gen := 0; gen < eng.Generations; gen++ {
		// 1. Оценка популяции (Параллельно)
		var wg sync.WaitGroup
		wg.Add(len(population))
		for _, ind := range population {
			go func(individual *algorithm.Schedule) {
				defer wg.Done()
				eng.Evaluator.CalculateFitness(individual)
			}(ind)
		}
		wg.Wait()

		// 2. Сортировка
		sort.Slice(population, func(i, j int) bool {
			return population[i].Fitness > population[j].Fitness
		})

		bestInd := population[0]

		// === ОПРЕДЕЛЕНИЕ СТРАТЕГИИ МУТАЦИИ ===
		var mutateToApply MutationFunc
		var currentMutationRate float64

		// Обновляем общий лучший фитнес
		if bestInd.Fitness > bestFitnessOverall {
			bestFitnessOverall = bestInd.Fitness
		}

		mutateToApply, currentMutationRate, stagnantGenerations, recoveryCounter = eng.determineMutationStrategy(
			bestInd.Fitness,
			bestFitnessOverall,
			stagnantGenerations,
			recoveryCounter,
		)
		// =====================================

		// Логирование прогресса
		if gen%20 == 0 || gen == eng.Generations-1 {
			log.Printf("[Gen %3d] Best Fit: %.4f | Stag: %2d | MutRate: %.3f",
				gen, bestInd.Fitness, stagnantGenerations, currentMutationRate)
		}

		// 3. Селекция и Скрещивание
		newPop := make([]*algorithm.Schedule, 0, eng.PopulationSize)
		eliteCount := int(float64(eng.PopulationSize) * 0.1)
		if shockMode {
			eliteCount = 2 // При шоке оставляем только самого лучшего, остальных в топку
		}
		newPop = append(newPop, population[:eliteCount]...)

		for len(newPop) < eng.PopulationSize {
			p1 := tournamentSelect(population, 3) // Турнир размером 3
			p2 := tournamentSelect(population, 3)

			child := eng.crossover(p1, p2)
			mutateToApply(child, currentMutationRate)
			newPop = append(newPop, child)
		}
		population = newPop
	}

	// Финальная переоценка первого (чтобы точно вернуть свежий Фитнес)
	eng.Evaluator.CalculateFitness(population[0])
	return population[0], nil
}

// determineMutationStrategy анализирует прогресс и решает, какую мутацию применить.
// Возвращает:
// 1. mutationFn - функция мутации (eng.mutate или eng.softMutate)
// 2. mutationRate - рассчитанный шанс мутации
// 3. newStagnantGens - обновленный счетчик стагнации
// 4. newRecoveryCounter - обновленный счетчик восстановления
func (eng *GeneticEngine) determineMutationStrategy(
	bestFitness float64,
	bestFitnessOverall float64,
	stagnantGens int,
	recoveryCounter int,
) (mutationFn MutationFunc, mutationRate float64, newStagnantGens int, newRecoveryCounter int) {

	// --- Фаза 1: Оптимизация (валидное решение найдено) ---
	if bestFitness >= 1.0 {
		// Мы в режиме улучшения мягких ограничений.
		// Логика нагрева/шока здесь не нужна, она слишком агрессивна.
		// Просто используем мягкую мутацию с фиксированным шансом.
		return eng.softMutate, 0.15, stagnantGens, recoveryCounter
	}

	// --- Фаза 2: Поиск (ищем валидное решение) ---
	// Здесь работает твоя логика "нагрев и шок"

	// Если мы в фазе восстановления после шока - ничего не делаем
	if recoveryCounter > 0 {
		return eng.mutate, eng.MutationRate, 0, recoveryCounter - 1
	}

	// Проверяем стагнацию
	var isStagnating bool
	if bestFitness > bestFitnessOverall+0.0000001 {
		// Прогресс есть! Сбрасываем стагнацию.
		newStagnantGens = 0
		isStagnating = false
	} else {
		// Прогресса нет, увеличиваем счетчик.
		newStagnantGens = stagnantGens + 1
		isStagnating = true
	}

	// Расчет скорости мутации на основе стагнации
	currentMutationRate := eng.MutationRate   // Базовая ставка
	if isStagnating && newStagnantGens > 10 { // Нагрев
		heatStep := float64((newStagnantGens-10)/10) * 0.01
		currentMutationRate = eng.MutationRate + heatStep
		if currentMutationRate > 0.15 {
			currentMutationRate = 0.15
		}
	}

	// ШОКОВАЯ ТЕРАПИЯ
	if isStagnating && newStagnantGens > 80 {
		log.Printf("!!! SHOCK THERAPY (Hard search) !!!")
		return eng.mutate, 0.3, 0, max(int(float64(eng.Generations)*0.05), 20)
	}

	return eng.mutate, currentMutationRate, newStagnantGens, 0
}

// Вспомогательные методы (crossover, mutate, createRandomIndividual)

// createRandomIndividual создает случайную хромосому
func (eng *GeneticEngine) createRandomIndividual() *algorithm.Schedule {
	assignments := make([]*algorithm.Assignment, len(eng.Classes))

	for i, cls := range eng.Classes {
		// Считаем размер группы ДО выбора аудитории
		var groupIDs []uint
		studentsCount := 0
		for _, g := range cls.Groups {
			groupIDs = append(groupIDs, g.ID)
			studentsCount += g.Size
		}

		// === ЭВРИСТИКА ВМЕСТИМОСТИ ===
		// Собираем список только тех аудиторий, куда влезает эта толпа
		var validRooms []uint
		for _, roomID := range eng.RoomIDs {
			room := eng.Evaluator.Context.RoomsMap[roomID]
			if room.Capacity >= studentsCount {
				validRooms = append(validRooms, roomID)
			}
		}

		// Если вдруг (баг данных) нет ни одной такой аудитории, берем любую (чтобы не упасть)
		var rndRoom uint
		if len(validRooms) > 0 {
			rndRoom = validRooms[rand.Intn(len(validRooms))]
		} else {
			rndRoom = eng.RoomIDs[rand.Intn(len(eng.RoomIDs))]
		}
		// =============================

		rndSlot := eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]

		assignments[i] = &algorithm.Assignment{
			ClassID:       cls.ID,
			RoomID:        rndRoom, // Теперь это гарантированно подходящая аудитория!
			SlotID:        rndSlot,
			InstructorID:  cls.InstructorID,
			GroupIDs:      groupIDs,
			StudentsCount: studentsCount,
		}
	}
	return algorithm.NewSchedule(assignments)
}

func (eng *GeneticEngine) crossover(p1, p2 *algorithm.Schedule) *algorithm.Schedule {
	childAssigns := make([]*algorithm.Assignment, len(p1.Assignments))

	for i := 0; i < len(p1.Assignments); i++ {
		var parentAssign *algorithm.Assignment

		// Uniform Crossover: случайно выбираем родителя для каждого гена
		if rand.Float64() < 0.5 {
			parentAssign = p1.Assignments[i]
		} else {
			parentAssign = p2.Assignments[i]
		}

		childAssigns[i] = &algorithm.Assignment{
			ClassID:       parentAssign.ClassID,
			RoomID:        parentAssign.RoomID,
			SlotID:        parentAssign.SlotID,
			InstructorID:  parentAssign.InstructorID,
			GroupIDs:      append([]uint(nil), parentAssign.GroupIDs...),
			StudentsCount: parentAssign.StudentsCount,
		}
	}
	return algorithm.NewSchedule(childAssigns)
}

// mutate случайным образом изменяет гены с заданным шансом (rate)
func (eng *GeneticEngine) mutate(schedule *algorithm.Schedule, rate float64) {
	for _, assignment := range schedule.Assignments {
		if rand.Float64() < rate {
			if rand.Float64() < 0.5 {
				assignment.SlotID = eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]
			} else {
				// === МУТИРУЕМ ТОЛЬКО В ПОДХОДЯЩИЕ АУДИТОРИИ ===
				var validRooms []uint
				for _, roomID := range eng.RoomIDs {
					if eng.Evaluator.Context.RoomsMap[roomID].Capacity >= assignment.StudentsCount {
						validRooms = append(validRooms, roomID)
					}
				}
				if len(validRooms) > 0 {
					assignment.RoomID = validRooms[rand.Intn(len(validRooms))]
				}
			}
		}
	}
}

// softMutate пытается переместить одно случайное занятие в новый случайный слот,
// но только если это не создает жестких конфликтов.
func (eng *GeneticEngine) softMutate(schedule *algorithm.Schedule, rate float64) {
	if rand.Float64() >= rate {
		return // Мутация не сработала по вероятности
	}

	// 1. Выбираем случайное занятие для перемещения
	if len(schedule.Assignments) == 0 {
		return
	}
	assignIndex := rand.Intn(len(schedule.Assignments))
	assignToMove := schedule.Assignments[assignIndex]

	// 2. Сохраняем его текущее положение, чтобы можно было откатиться
	originalSlotID := assignToMove.SlotID

	// 3. Выбираем новый случайный слот
	targetSlotID := eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]

	if originalSlotID == targetSlotID {
		return // Перемещать в то же место нет смысла
	}

	// 4. Временно перемещаем занятие
	assignToMove.SlotID = targetSlotID

	// 5. Проверяем, не сломали ли мы всё (жесткие ограничения)
	hardConflicts, _ := eng.Evaluator.CountConflicts(schedule) // Считает и жесткие, и мягкие

	// 6. Принимаем решение
	if hardConflicts > 0 {
		// Перемещение создало конфликт! Откатываемся.
		assignToMove.SlotID = originalSlotID
	}
	// Если hardConflicts == 0, то ничего не делаем, оставляя занятие на новом месте.
	// Изменение принято!
}

func tournamentSelect(population []*algorithm.Schedule, k int) *algorithm.Schedule {
	// Выбираем первого участника как текущего победителя
	best := population[rand.Intn(len(population))]

	// Проводим турнир
	for i := 1; i < k; i++ {
		contender := population[rand.Intn(len(population))]
		if contender.Fitness > best.Fitness {
			best = contender
		}
	}
	return best
}
