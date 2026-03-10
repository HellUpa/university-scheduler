package greedy

import (
	"fmt"
	"scheduler/internal/algorithm"
	"scheduler/internal/domain"
	"sort"

	"gorm.io/gorm"
)

type GreedyEngine struct {
	DB *gorm.DB

	// Контекст (загружаем так же, как в ГА)
	Classes []domain.CourseClass
	RoomIDs []uint
	SlotIDs []uint

	ClassesMap map[uint]*domain.CourseClass
	Evaluator  *algorithm.Evaluator
}

func NewEngine(db *gorm.DB) *GreedyEngine {
	return &GreedyEngine{
		DB: db,
	}
}

func (eng *GreedyEngine) Prepare() error {
	if err := eng.DB.Preload("Groups").Preload("Subject").Preload("Instructor").Find(&eng.Classes).Error; err != nil {
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

	eng.Evaluator = algorithm.NewEvaluator(rooms, slots, eng.Classes)

	for _, r := range rooms {
		eng.RoomIDs = append(eng.RoomIDs, r.ID)
	}
	for _, s := range slots {
		eng.SlotIDs = append(eng.SlotIDs, s.ID)
	}

	return nil
}

func (eng *GreedyEngine) Run(isPrepared bool) (*algorithm.Schedule, error) {
	if !isPrepared {
		if err := eng.Prepare(); err != nil {
			return nil, err
		}
	}

	// 1. ЭВРИСТИКА: Сортируем занятия от сложных к простым.
	// Большие группы и лекции нужно ставить в первую очередь.
	sort.Slice(eng.Classes, func(i, j int) bool {
		// Сначала сравниваем по размеру (кол-во студентов)
		sizeI := 0
		for _, g := range eng.Classes[i].Groups {
			sizeI += g.Size
		}

		sizeJ := 0
		for _, g := range eng.Classes[j].Groups {
			sizeJ += g.Size
		}

		if sizeI != sizeJ {
			return sizeI > sizeJ // По убыванию
		}

		// Затем по типу (Лекции приоритетнее)
		if eng.Classes[i].IsLecture && !eng.Classes[j].IsLecture {
			return true
		}
		if !eng.Classes[i].IsLecture && eng.Classes[j].IsLecture {
			return false
		}

		return eng.Classes[i].ID < eng.Classes[j].ID // Стабильная сортировка по ID
	})

	assignments := make([]*algorithm.Assignment, len(eng.Classes))

	// Структуры для отслеживания занятости (чтобы не создавать коллизий, если возможно)
	roomUsage := make(map[struct{ S, R uint }]bool)
	instructorUsage := make(map[struct{ S, I uint }]bool)
	groupUsage := make(map[struct{ S, G uint }]bool)

	// 2. РАСПРЕДЕЛЕНИЕ
	for i, cls := range eng.Classes {
		placed := false

		var groupIDs []uint
		studentsCount := 0
		for _, g := range cls.Groups {
			groupIDs = append(groupIDs, g.ID)
			studentsCount += g.Size
		}

		// Ищем первый подходящий слот и аудиторию
		// Сначала перебираем дни (слоты), затем аудитории (чтобы заполнять дни равномерно)
		for _, slotID := range eng.SlotIDs {
			for _, roomID := range eng.RoomIDs {

				// Проверяем, свободна ли аудитория
				if roomUsage[struct{ S, R uint }{slotID, roomID}] {
					continue
				}

				// Хватает ли места в аудитории?
				room := eng.Evaluator.Context.RoomsMap[roomID]
				if room.Capacity < studentsCount {
					continue
				}

				// Желательно (Soft Constraint), но в жадном алгоритме сделаем это строгим правилом для начала:
				// if room.Type != cls.RequiredRoomType { continue }

				// Свободен ли препод?
				if instructorUsage[struct{ S, I uint }{slotID, cls.InstructorID}] {
					continue
				}

				// Свободны ли все группы?
				groupsBusy := false
				for _, group := range cls.Groups { // <--- Итерируем сам объект
					if groupUsage[struct{ S, G uint }{slotID, group.ID}] {
						groupsBusy = true
						break
					}
				}
				if groupsBusy {
					continue
				}

				// БИНГО! Нашли место. Бронируем его!
				roomUsage[struct{ S, R uint }{slotID, roomID}] = true
				instructorUsage[struct{ S, I uint }{slotID, cls.InstructorID}] = true
				for _, group := range cls.Groups {
					groupUsage[struct{ S, G uint }{slotID, group.ID}] = true
				}

				assignments[i] = &algorithm.Assignment{
					ClassID:       cls.ID,
					RoomID:        roomID,
					SlotID:        slotID,
					InstructorID:  cls.InstructorID,
					GroupIDs:      groupIDs,
					StudentsCount: studentsCount,
				}
				placed = true
				break // Переходим к следующему занятию
			}
			if placed {
				break
			} // Выходим из цикла слотов
		}

		// Если мы перебрали все слоты и аудитории, но так и не смогли поставить занятие без конфликтов
		if !placed {
			// Тупик жадного алгоритма!
			// Втыкаем его в первый попавшийся слот и первую попавшуюся аудиторию,
			// создавая жесткий конфликт (Evaluator потом это оценит).
			fmt.Printf("Greedy Algorithm WARNING: Forced collision for ClassID %d\n", cls.ID)
			assignments[i] = &algorithm.Assignment{
				ClassID:       cls.ID,
				RoomID:        0,
				SlotID:        0,
				InstructorID:  cls.InstructorID,
				GroupIDs:      groupIDs,
				StudentsCount: studentsCount,
			}
		}
	}

	// 3. СОЗДАЕМ И ОЦЕНИВАЕМ РАСПИСАНИЕ
	schedule := algorithm.NewSchedule(assignments)
	eng.Evaluator.CalculateFitness(schedule)

	return schedule, nil
}
