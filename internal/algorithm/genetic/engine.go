package genetic

import (
	"log"
	"math/rand"
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
	Evaluator  *Evaluator
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
		Preload("Subject").    // <--- ДОБАВИЛИ
		Preload("Instructor"). // <--- ДОБАВИЛИ
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

	eng.Evaluator = NewEvaluator(rooms, slots, eng.Classes)

	for _, r := range rooms {
		eng.RoomIDs = append(eng.RoomIDs, r.ID)
	}
	for _, s := range slots {
		eng.SlotIDs = append(eng.SlotIDs, s.ID)
	}

	return nil
}

func (eng *GeneticEngine) Run() (*Individual, error) {
	if err := eng.Prepare(); err != nil {
		return nil, err
	}

	population := make([]*Individual, eng.PopulationSize)
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
			go func(individual *Individual) {
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

		// Если мы в фазе восстановления - просто уменьшаем счетчик и ничего не делаем
		if recoveryCounter > 0 {
			recoveryCounter--
			stagnantGenerations = 0 // Сбрасываем стагнацию, пока восстанавливаемся
		} else {
			// Обычный режим: проверяем стагнацию
			if bestInd.Fitness > bestFitnessOverall+0.001 {
				bestFitnessOverall = bestInd.Fitness
				stagnantGenerations = 0
			} else {
				stagnantGenerations++
			}

			// Если застряли на 15 поколений -> Готовим УДАР
			if stagnantGenerations >= 15 {
				shockMode = true
				stagnantGenerations = 0 // Сброс
				recoveryCounter = 20    // Даем 20 поколений на восстановление после удара
				log.Printf("[Gen %d] !!! STAGNATION DETECTED -> SHOCK THERAPY !!!", gen)
			}
		}

		// Определяем текущую мутацию
		currentMutationRate := eng.MutationRate
		if shockMode {
			currentMutationRate = 0.5 // Взрываем 50% генов!
			shockMode = false         // Удар длится всего 1 поколение
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
		newPop := make([]*Individual, 0, eng.PopulationSize)
		eliteCount := int(float64(eng.PopulationSize) * 0.1)
		if currentMutationRate > 0.4 {
			eliteCount = 1 // При шоке оставляем только самого лучшего, остальных в топку
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
func (eng *GeneticEngine) createRandomIndividual() *Individual {
	genes := make([]*Gene, len(eng.Classes))

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

		genes[i] = &Gene{
			ClassID:       cls.ID,
			RoomID:        rndRoom,
			SlotID:        rndSlot,
			InstructorID:  cls.InstructorID,
			GroupIDs:      groupIDs,
			StudentsCount: studentsCount,
		}
	}

	return NewIndividual(genes)
}

// crossover выполняет одноточечное скрещивание
func (eng *GeneticEngine) crossover(p1, p2 *Individual) *Individual {
	point := rand.Intn(len(p1.Genes))
	childGenes := make([]*Gene, len(p1.Genes))

	for i := 0; i < len(p1.Genes); i++ {
		var parentGene *Gene
		if i < point {
			parentGene = p1.Genes[i]
		} else {
			parentGene = p2.Genes[i]
		}

		// ГЛУБОКОЕ КОПИРОВАНИЕ (Deep Copy), чтобы мутация не затрагивала родителей
		childGenes[i] = &Gene{
			ClassID:       parentGene.ClassID,
			RoomID:        parentGene.RoomID,
			SlotID:        parentGene.SlotID,
			InstructorID:  parentGene.InstructorID,
			GroupIDs:      append([]uint(nil), parentGene.GroupIDs...), // Копия среза
			StudentsCount: parentGene.StudentsCount,
		}
	}

	return NewIndividual(childGenes)
}

// mutate случайным образом изменяет гены с заданным шансом (rate)
func (eng *GeneticEngine) mutate(ind *Individual, rate float64) {
	for _, gene := range ind.Genes {
		if rand.Float64() < rate {
			if rand.Float64() < 0.5 {
				gene.SlotID = eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]
			} else {
				gene.RoomID = eng.RoomIDs[rand.Intn(len(eng.RoomIDs))]
			}
		}
	}
}
