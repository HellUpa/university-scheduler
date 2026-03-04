package genetic

import (
	"math/rand"
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
	Classes   []domain.CourseClass
	RoomIDs   []uint
	SlotIDs   []uint
	Evaluator *Evaluator
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
	// Загрузка данных (Preload для связей)
	if err := eng.DB.Preload("Groups").Find(&eng.Classes).Error; err != nil {
		return err
	}

	var rooms []domain.Room
	if err := eng.DB.Find(&rooms).Error; err != nil {
		return err
	}

	var slots []domain.TimeSlot
	if err := eng.DB.Find(&slots).Error; err != nil {
		return err
	}

	eng.Evaluator = NewEvaluator(rooms, slots)

	// Кэшируем ID для рандома
	for _, r := range rooms {
		eng.RoomIDs = append(eng.RoomIDs, r.ID)
	}
	for _, s := range slots {
		eng.SlotIDs = append(eng.SlotIDs, s.ID)
	}

	rand.Seed(time.Now().UnixNano())
	return nil
}

func (eng *GeneticEngine) Run() (*Individual, error) {
	if err := eng.Prepare(); err != nil {
		return nil, err
	}

	// 1. Инициализация
	population := make([]*Individual, eng.PopulationSize)
	for i := 0; i < eng.PopulationSize; i++ {
		population[i] = eng.createRandomIndividual()
	}

	// 2. Эволюция
	for gen := 0; gen < eng.Generations; gen++ {

		// --- ПАРАЛЛЕЛЬНАЯ ОЦЕНКА (Concurrent Evaluation) ---
		var wg sync.WaitGroup
		wg.Add(len(population))

		for _, ind := range population {
			go func(individual *Individual) {
				defer wg.Done()
				individual.Fitness = eng.Evaluator.CalculateFitness(individual)
			}(ind)
		}
		wg.Wait()
		// ----------------------------------------------------

		// Сортировка
		sort.Slice(population, func(i, j int) bool {
			return population[i].Fitness > population[j].Fitness
		})

		bestFit := population[0].Fitness
		// Логгируем, но не каждое поколение, чтоб не засорять консоль
		if gen%10 == 0 || bestFit > 0.99 {
			// fmt.Printf("Gen %d: Best Fitness = %.4f\n", gen, bestFit)
		}

		if bestFit > 0.999 {
			break
		}

		// Селекция и скрещивание
		newPop := make([]*Individual, 0, eng.PopulationSize)

		// Элитаризм (10%)
		eliteCount := int(float64(eng.PopulationSize) * 0.1)
		newPop = append(newPop, population[:eliteCount]...)

		// Добираем остальных
		for len(newPop) < eng.PopulationSize {
			p1 := population[rand.Intn(len(population)/2)] // Из лучшей половины
			p2 := population[rand.Intn(len(population)/2)]

			child := eng.crossover(p1, p2)
			eng.mutate(child)
			newPop = append(newPop, child)
		}
		population = newPop
	}

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

// mutate случайным образом изменяет гены
func (eng *GeneticEngine) mutate(ind *Individual) {
	for _, gene := range ind.Genes {
		if rand.Float64() < eng.MutationRate {
			// С вероятностью 50% меняем либо слот, либо аудиторию
			if rand.Float64() < 0.5 {
				gene.SlotID = eng.SlotIDs[rand.Intn(len(eng.SlotIDs))]
			} else {
				gene.RoomID = eng.RoomIDs[rand.Intn(len(eng.RoomIDs))]
			}
		}
	}
}
