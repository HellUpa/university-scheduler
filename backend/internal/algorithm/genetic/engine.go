package genetic

import (
	"log"
	"math/rand"
	"runtime"
	"scheduler/internal/algorithm"
	"scheduler/internal/domain"
	"sort"
	"sync"

	"gorm.io/gorm"
)

// MutationFunc - это тип для наших функций мутации (жесткой и мягкой)
type MutationFunc func(*algorithm.Schedule, float64)

// Для callback
type ProgressFunc func(gen int, fitness float64, mutRate float64)

type GeneticEngine struct {
	DB       *gorm.DB
	IsSilent bool

	// Главные параметры
	PopulationSize   int
	Generations      int
	BaseMutationRate float64

	// Дополнительные параметры
	EliteSize      float64
	TournamentSize int
	CrossoverRate  float64
	// Мягкая мутация (без жестких конфликтов)
	IsSoftMutationEnabled bool
	SoftMutationRate      float64
	SoftMutationAttempts  int
	// Нагрев мутации при стагнации
	HeatStagnantCount int
	HeatStepScale     float64
	// Шоковая терапия, если нагрев не помогает
	ShockStagnantCount    int
	ShockMutationRate     float64
	ShockMinRecoveryCount int
	ShockRecoveryScale    float64

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
		DB:               db,
		IsSilent:         false,
		PopulationSize:   100,
		Generations:      200,
		BaseMutationRate: 0.001,

		EliteSize:             0.05,
		TournamentSize:        3,
		CrossoverRate:         0.8,
		IsSoftMutationEnabled: false,
		SoftMutationRate:      0.10,
		SoftMutationAttempts:  10,
		HeatStagnantCount:     10,
		HeatStepScale:         0.1,
		ShockStagnantCount:    80,
		ShockMutationRate:     0.2,
		ShockMinRecoveryCount: 20,
		ShockRecoveryScale:    0.05,
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

	eng.Evaluator = algorithm.NewEvaluator(rooms, slots, eng.Classes)

	for _, r := range rooms {
		eng.RoomIDs = append(eng.RoomIDs, r.ID)
	}
	for _, s := range slots {
		eng.SlotIDs = append(eng.SlotIDs, s.ID)
	}

	return nil
}

func (eng *GeneticEngine) Run(onProgress ProgressFunc) (*algorithm.Schedule, error) {
	if err := eng.Prepare(); err != nil {
		return nil, err
	}

	population := make([]*algorithm.Schedule, eng.PopulationSize)

	for i := 0; i < eng.PopulationSize; i++ {
		population[i] = eng.createRandomIndividual()
	}

	bestPenaltyOverall := 0.0
	stagnantGenerations := 0
	recoveryCounter := 0 // Счетчик поколений восстановления (иммунитет)
	// =========================================

	for gen := 0; gen < eng.Generations; gen++ {
		// 1. Оценка популяции (Параллельно)
		var wg sync.WaitGroup
		numCPUs := runtime.NumCPU()
		sem := make(chan struct{}, numCPUs)

		for _, ind := range population {
			wg.Add(1)

			// Захватываем слот в семафоре
			sem <- struct{}{}
			go func(individual *algorithm.Schedule) {
				defer wg.Done()
				defer func() { <-sem }()
				eng.Evaluator.CalculateFitness(individual)
			}(ind)
		}
		wg.Wait()

		// 2. Сортировка
		sort.Slice(population, func(i, j int) bool {
			return population[i].InternalPenalty < population[j].InternalPenalty
		})

		bestInd := population[0]

		// === ОПРЕДЕЛЕНИЕ СТРАТЕГИИ МУТАЦИИ ===
		var mutateToApply MutationFunc
		var currentMutationRate float64

		// Обновляем общий лучший фитнес
		if bestInd.InternalPenalty < bestPenaltyOverall {
			bestPenaltyOverall = bestInd.InternalPenalty
		}

		mutateToApply, currentMutationRate, stagnantGenerations, recoveryCounter = eng.determineMutationStrategy(
			bestInd.HardConflicts == 0, // проверка на валидность расписания
			bestInd.InternalPenalty,
			bestPenaltyOverall,
			stagnantGenerations,
			recoveryCounter,
		)
		// =====================================

		if onProgress != nil {
			onProgress(gen, bestInd.UserFitness, currentMutationRate)
		}

		// Логирование прогресса
		if !eng.IsSilent {
			if gen%20 == 0 || gen == eng.Generations-1 {
				log.Printf("[Gen %3d] Best Penalty: %.1f | Stag: %2d | MutRate: %.3f",
					gen, bestInd.InternalPenalty, stagnantGenerations, currentMutationRate)
			}
		}

		// 3. Селекция и Скрещивание
		newPop := make([]*algorithm.Schedule, 0, eng.PopulationSize)
		eliteCount := int(float64(eng.PopulationSize) * eng.EliteSize)
		newPop = append(newPop, population[:eliteCount]...)

		for len(newPop) < eng.PopulationSize {
			p1 := tournamentSelect(population, eng.TournamentSize) // Турнир размером 3
			p2 := tournamentSelect(population, eng.TournamentSize)

			child := eng.crossover(p1, p2, eng.CrossoverRate)
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
	isValid bool,
	bestFitness float64,
	bestFitnessOverall float64,
	stagnantGens int,
	recoveryCounter int,
) (mutationFn MutationFunc, mutationRate float64, newStagnantGens int, newRecoveryCounter int) {

	// --- Фаза 1: Оптимизация (валидное решение найдено) ---
	if eng.IsSoftMutationEnabled && isValid {
		// Мы в режиме улучшения мягких ограничений.
		// Просто используем мягкую мутацию с фиксированным шансом.
		return eng.softMutate, eng.SoftMutationRate, stagnantGens, recoveryCounter
	}

	// --- Фаза 2: Поиск (ищем валидное решение) ---

	// Если мы в фазе восстановления после шока - ничего не делаем
	if recoveryCounter > 0 {
		return eng.mutate, eng.BaseMutationRate, 0, recoveryCounter - 1
	}

	// Проверяем стагнацию
	var isStagnating bool
	if bestFitnessOverall > bestFitness {
		// Прогресс есть! Сбрасываем стагнацию.
		newStagnantGens = 0
		isStagnating = false
	} else {
		// Прогресса нет, увеличиваем счетчик.
		newStagnantGens = stagnantGens + 1
		isStagnating = true
	}

	// Расчет скорости мутации на основе стагнации
	currentMutationRate := eng.BaseMutationRate                  // Базовая ставка
	if isStagnating && newStagnantGens > eng.HeatStagnantCount { // Нагрев
		heatStep := float64((newStagnantGens)) * eng.HeatStepScale
		currentMutationRate = eng.BaseMutationRate + (eng.BaseMutationRate * heatStep)
	}

	// ШОКОВАЯ ТЕРАПИЯ
	if isStagnating && newStagnantGens > eng.ShockStagnantCount {
		return eng.mutate, eng.ShockMutationRate, 0, max(int(float64(eng.Generations)*eng.ShockRecoveryScale), eng.ShockMinRecoveryCount)
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
			room := eng.Evaluator.Data.RoomsMap[roomID]
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

func (eng *GeneticEngine) crossover(p1, p2 *algorithm.Schedule, crossoverRate float64) *algorithm.Schedule {
	childAssigns := make([]*algorithm.Assignment, len(p1.Assignments))

	if rand.Float64() < crossoverRate {
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
	} else {
		var parent *algorithm.Schedule
		if rand.Float64() < 0.5 {
			parent = p1
		} else {
			parent = p2
		}
		return parent
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
					if eng.Evaluator.Data.RoomsMap[roomID].Capacity >= assignment.StudentsCount {
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
// но только если это не создает жестких конфликтов. Делает несколько попыток
func (eng *GeneticEngine) softMutate(schedule *algorithm.Schedule, rate float64) {
	if rand.Float64() >= rate {
		return
	}

	// Попробуем сделать несколько попыток найти удачную мутацию, прежде чем сдаться
	for range eng.SoftMutationAttempts {
		assignIndex := rand.Intn(len(schedule.Assignments))
		assignToMove := schedule.Assignments[assignIndex]

		originalSlotID := assignToMove.SlotID
		targetSlotID := eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]

		if originalSlotID == targetSlotID {
			continue
		}

		// --- КЛЮЧЕВОЕ УЛУЧШЕНИЕ ---
		// Быстрая проверка, не занят ли этот слот уже этой же группой или преподавателем
		// Это отсеет самые очевидные жесткие конфликты до полной дорогой проверки.
		isOccupied := false
		for _, otherAssign := range schedule.Assignments {
			if otherAssign.SlotID == targetSlotID {
				if otherAssign.RoomID == assignToMove.RoomID {
					isOccupied = true // Аудитория занята
					break
				}
				if otherAssign.InstructorID == assignToMove.InstructorID {
					isOccupied = true // Преподаватель занят
					break
				}
				for _, groupID := range assignToMove.GroupIDs {
					for _, otherGroupID := range otherAssign.GroupIDs {
						if groupID == otherGroupID {
							isOccupied = true // Группа занята
							break
						}
					}
					if isOccupied {
						break
					}
				}
			}
			if isOccupied {
				break
			}
		}

		if isOccupied {
			continue // Слот очевидно занят, пробуем еще раз
		}
		// -------------------------

		assignToMove.SlotID = targetSlotID

		// 5. Проверяем ТОЛЬКО жесткие конфликты
		hardConflicts, _ := eng.Evaluator.CountConflicts(schedule)

		// 6. Принимаем решение
		if hardConflicts > 0 {
			// Перемещение создало жесткий конфликт! Откатываемся.
			assignToMove.SlotID = originalSlotID
			// и продолжаем цикл, чтобы попробовать еще раз
		} else {
			// Конфликтов нет! Мутация успешна.
			// Теперь выходим из функции.
			return
		}
	}
}

func tournamentSelect(population []*algorithm.Schedule, k int) *algorithm.Schedule {
	// Выбираем первого участника как текущего победителя
	best := population[rand.Intn(len(population))]

	// Проводим турнир
	for i := 1; i < k; i++ {
		contender := population[rand.Intn(len(population))]
		if contender.InternalPenalty < best.InternalPenalty {
			best = contender
		}
	}
	return best
}
