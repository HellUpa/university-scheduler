package genetic

import (
	"math/rand"
	"scheduler/internal/algorithm"
	"scheduler/internal/domain"
	"sort"
	"sync"
	"time"

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

// Run оркестрирует параллельный запуск (Островная модель)
func (eng *GeneticEngine) Run(instances int) (*algorithm.Schedule, error) {
	// Подготовка данных (Обращение к БД) делается ОДИН раз для всех!
	if err := eng.Prepare(); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	results := make(chan *algorithm.Schedule, instances)

	// Запускаем N независимых эволюций параллельно
	for i := 0; i < instances; i++ {
		wg.Add(1)
		go func(instanceID int) {
			defer wg.Done()
			seed := time.Now().UnixNano() + int64(instanceID*1000)
			localRand := rand.New(rand.NewSource(seed))

			// Передаем генератор внутрь инстанса
			best := eng.runInstance(localRand)
			results <- best
		}(i)
	}

	// Ждем, пока все острова закончат эволюцию
	wg.Wait()
	close(results)

	// Выбираем глобального победителя
	var globalBest *algorithm.Schedule
	for best := range results {
		if globalBest == nil || best.Fitness > globalBest.Fitness {
			globalBest = best
		}
	}

	return globalBest, nil
}

// runInstance - логика одной изолированной популяции (одного "острова")
func (eng *GeneticEngine) runInstance(rng *rand.Rand) *algorithm.Schedule {
	population := make([]*algorithm.Schedule, eng.PopulationSize)
	for i := 0; i < eng.PopulationSize; i++ {
		// Передаем локальный генератор
		population[i] = eng.createRandomIndividual(rng)
	}

	bestFitnessOverall := 0.0
	stagnantGenerations := 0
	shockMode := false
	recoveryCounter := 0

	for gen := 0; gen < eng.Generations; gen++ {
		for _, ind := range population {
			eng.Evaluator.CalculateFitness(ind)
		}

		sort.Slice(population, func(i, j int) bool {
			return population[i].Fitness > population[j].Fitness
		})

		bestInd := population[0]

		if bestInd.Fitness > 1.2 {
			break
		}

		if recoveryCounter > 0 {
			recoveryCounter--
			stagnantGenerations = 0
		} else {
			if bestInd.Fitness > bestFitnessOverall+0.001 {
				bestFitnessOverall = bestInd.Fitness
				stagnantGenerations = 0
			} else {
				stagnantGenerations++
			}

			if stagnantGenerations >= 15 {
				shockMode = true
				stagnantGenerations = 0
				recoveryCounter = 20
			}
		}

		currentMutationRate := eng.MutationRate
		if shockMode {
			currentMutationRate = 0.5
			shockMode = false
		}

		newPop := make([]*algorithm.Schedule, 0, eng.PopulationSize)

		eliteCount := int(float64(eng.PopulationSize) * 0.1)
		if currentMutationRate > 0.4 {
			eliteCount = 1
		}
		newPop = append(newPop, population[:eliteCount]...)

		for len(newPop) < eng.PopulationSize {
			// Используем rng.Intn вместо rand.Intn
			p1 := population[rng.Intn(len(population)/2)]
			p2 := population[rng.Intn(len(population)/2)]

			child := eng.crossover(p1, p2, rng)         // Передаем rng в кроссовер
			eng.mutate(child, currentMutationRate, rng) // Передаем rng в мутацию
			newPop = append(newPop, child)
		}
		population = newPop
	}

	eng.Evaluator.CalculateFitness(population[0])
	return population[0]
}

// === ОБНОВЛЯЕМ ФУНКЦИИ ГЕНЕТИКИ ===

func (eng *GeneticEngine) createRandomIndividual(rng *rand.Rand) *algorithm.Schedule {
	assignments := make([]*algorithm.Assignment, len(eng.Classes))

	for i, cls := range eng.Classes {
		// Используем rng вместо rand
		rndRoom := eng.RoomIDs[rng.Intn(len(eng.RoomIDs))]
		rndSlot := eng.SlotIDs[rng.Intn(len(eng.SlotIDs))]

		var groupIDs []uint
		studentsCount := 0
		for _, g := range cls.Groups {
			groupIDs = append(groupIDs, g.ID)
			studentsCount += g.Size
		}

		assignments[i] = &algorithm.Assignment{
			ClassID:       cls.ID,
			RoomID:        rndRoom,
			SlotID:        rndSlot,
			InstructorID:  cls.InstructorID,
			GroupIDs:      groupIDs,
			StudentsCount: studentsCount,
		}
	}
	return algorithm.NewSchedule(assignments)
}

func (eng *GeneticEngine) crossover(p1, p2 *algorithm.Schedule, rng *rand.Rand) *algorithm.Schedule {
	point := rng.Intn(len(p1.Assignments)) // Используем rng
	childAssigns := make([]*algorithm.Assignment, len(p1.Assignments))

	for i := 0; i < len(p1.Assignments); i++ {
		var parentAssign *algorithm.Assignment
		if i < point {
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

func (eng *GeneticEngine) mutate(schedule *algorithm.Schedule, rate float64, rng *rand.Rand) {
	for _, assignment := range schedule.Assignments {
		if rng.Float64() < rate { // Используем rng
			if rng.Float64() < 0.5 {
				assignment.SlotID = eng.SlotIDs[rng.Intn(len(eng.SlotIDs))]
			} else {
				assignment.RoomID = eng.RoomIDs[rng.Intn(len(eng.RoomIDs))]
			}
		}
	}
}
