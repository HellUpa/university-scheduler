package genetic

import (
	"scheduler/internal/domain"
)

type Evaluator struct {
	RoomsMap map[uint]*domain.Room
	SlotsMap map[uint]*domain.TimeSlot
}

func NewEvaluator(rooms []domain.Room, slots []domain.TimeSlot) *Evaluator {
	rMap := make(map[uint]*domain.Room)
	sMap := make(map[uint]*domain.TimeSlot)

	for i := range rooms {
		rMap[rooms[i].ID] = &rooms[i]
	}
	for i := range slots {
		sMap[slots[i].ID] = &slots[i]
	}

	return &Evaluator{RoomsMap: rMap, SlotsMap: sMap}
}

// CalculateFitness вычисляет приспособленность
func (e *Evaluator) CalculateFitness(ind *Individual) float64 {
	penalty := 0.0

	// Карты занятости для проверки коллизий
	// Key: SlotID_EntityID
	roomUsage := make(map[struct{ S, R uint }]bool)
	instructorUsage := make(map[struct{ S, I uint }]bool)
	groupUsage := make(map[struct{ S, G uint }]bool)

	for _, gene := range ind.Genes {
		// 1. Вместимость
		if room, ok := e.RoomsMap[gene.RoomID]; ok {
			if room.Capacity < gene.StudentsCount {
				penalty += 10.0 // Штраф
			}
			// Проверка типа аудитории
			// if room.Type != ... { penalty += ... }
		}

		// 2. Коллизия Аудитории
		roomKey := struct{ S, R uint }{gene.SlotID, gene.RoomID}
		if roomUsage[roomKey] {
			penalty += 100.0 // Жесткий конфликт
		}
		roomUsage[roomKey] = true

		// 3. Коллизия Преподавателя
		instKey := struct{ S, I uint }{gene.SlotID, gene.InstructorID}
		if instructorUsage[instKey] {
			penalty += 100.0
		}
		instructorUsage[instKey] = true

		// 4. Коллизия Групп
		for _, gid := range gene.GroupIDs {
			grpKey := struct{ S, G uint }{gene.SlotID, gid}
			if groupUsage[grpKey] {
				penalty += 100.0
			}
			groupUsage[grpKey] = true
		}
	}

	return 1.0 / (1.0 + penalty)
}
