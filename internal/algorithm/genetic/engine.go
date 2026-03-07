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

	eng.Evaluator = algorithm.NewEvaluator(rooms, slots, eng.Classes)

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

		// === ЛОГИКА "УДАР И ВОССТАНОВЛЕНИЕ" ===

		currentMutationRate := eng.MutationRate

		// Если мы в фазе восстановления - просто уменьшаем счетчик и ничего не делаем
		if recoveryCounter > 0 {
			recoveryCounter--
			stagnantGenerations = 0 // Сбрасываем стагнацию, пока восстанавливаемся
		} else {
			// Обычный режим: проверяем стагнацию
			if bestInd.Fitness > bestFitnessOverall+0.001 {
				bestFitnessOverall = bestInd.Fitness
				stagnantGenerations = 0
				// Сброс мутации до базовой (Остывание)
				eng.MutationRate = 0.05
			} else {
				stagnantGenerations++
			}

			// Если застряли - начинаем ПЛАВНО нагревать
			if stagnantGenerations > 5 {
				// Каждые 5 поколений стагнации добавляем +0.01 к мутации
				heatStep := float64(stagnantGenerations/5) * 0.01
				currentMutationRate = 0.05 + heatStep

				// Ограничитель нагрева (чтобы не сжечь всё)
				if currentMutationRate > 0.2 {
					currentMutationRate = 0.2
				}
			}

			// ШОКОВАЯ ТЕРАПИЯ (Если нагрев не помог долгое время)
			if stagnantGenerations > 40 { // Только если ОЧЕНЬ долго стоим (40 поколений)
				currentMutationRate = 0.4 // Сильный удар
				shockMode = true
				stagnantGenerations = 0                                       // Сброс счетчика, чтобы начать нагрев заново
				recoveryCounter = max(int(float64(eng.Generations)*0.05), 20) // Даем время восстановиться, 5% от общего числа поколений, если больше 20
				log.Printf("[Gen %d] !!! SHOCK THERAPY !!!", gen)
			}
		}
		// =========================================

		// Логирование прогресса
		if gen%20 == 0 || gen == eng.Generations-1 {
			log.Printf("[Gen %3d] Best Fit: %.4f | Stag: %2d | MutRate: %.3f",
				gen, bestInd.Fitness, stagnantGenerations, currentMutationRate)
		}

		// Выход, если нашли расписание без коллизий и с максимумом бонусов
		// 1.0 = нет коллизий. 1.09+ = отличные бонусы.
		if bestInd.Fitness > 1.09 {
			log.Printf("Optimal solution found at generation %d!", gen)
			break
		}

		// 3. Селекция и Скрещивание
		newPop := make([]*algorithm.Schedule, 0, eng.PopulationSize)
		eliteCount := int(float64(eng.PopulationSize) * 0.1)
		if shockMode {
			eliteCount = 2 // При шоке оставляем только самого лучшего, остальных в топку
		}
		newPop = append(newPop, population[:eliteCount]...)

		for len(newPop) < eng.PopulationSize {
			p1 := population[rand.Intn(len(population)/2)]
			p2 := population[rand.Intn(len(population)/2)]

			child := eng.crossover(p1, p2)
			eng.mutate(child, currentMutationRate) // Применяем обычную или шоковую мутацию
			newPop = append(newPop, child)
		}
		population = newPop
	}

	// Финальная переоценка первого (чтобы точно вернуть свежий Фитнес)
	eng.Evaluator.CalculateFitness(population[0])
	return population[0], nil
}

// Вспомогательные методы (crossover, mutate, createRandomIndividual)

// createRandomIndividual создает случайную хромосому
func (eng *GeneticEngine) createRandomIndividual() *algorithm.Schedule {
	genes := make([]*algorithm.Assignment, len(eng.Classes))

	for i, cls := range eng.Classes {
		// Случайная аудитория и слот
		rndRoom := eng.RoomIDs[rand.Intn(len(eng.RoomIDs))]
		rndSlot := eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]

		// Собираем данные по группам
		var groupIDs []uint
		studentsCount := 0
		for _, g := range cls.Groups {
			groupIDs = append(groupIDs, g.ID)
			studentsCount += g.Size
		}

		genes[i] = &algorithm.Assignment{
			ClassID:       cls.ID,
			RoomID:        rndRoom,
			SlotID:        rndSlot,
			InstructorID:  cls.InstructorID,
			GroupIDs:      groupIDs,
			StudentsCount: studentsCount,
		}
	}

	return algorithm.NewSchedule(genes)
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
func (eng *GeneticEngine) mutate(ind *algorithm.Schedule, rate float64) {
	for _, gene := range ind.Assignments {
		if rand.Float64() < rate {
			if rand.Float64() < 0.5 {
				gene.SlotID = eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]
			} else {
				gene.RoomID = eng.RoomIDs[rand.Intn(len(eng.RoomIDs))]
			}
		}
	}
}
