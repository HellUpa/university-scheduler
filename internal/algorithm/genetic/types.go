package genetic

// Gene - одно конкретное назначение
type Gene struct {
	ClassID uint
	RoomID  uint
	SlotID  uint

	// Кэшированные поля для быстрого расчета (чтобы не лазить в БД в цикле)
	InstructorID  uint
	GroupIDs      []uint
	StudentsCount int
}

// Individual - Вариант расписания
type Individual struct {
	Genes   []*Gene
	Fitness float64
}

// Конструктор для упрощения
func NewIndividual(genes []*Gene) *Individual {
	return &Individual{
		Genes:   genes,
		Fitness: 0.0,
	}
}
