package transport

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"scheduler/internal/algorithm"
	"scheduler/internal/algorithm/genetic"
	"scheduler/internal/algorithm/greedy"
)

type Handler struct {
	DB *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{DB: db}
}

// Структура для приема параметров алгоритма из тела запроса (POST JSON)
type GenerateRequest struct {
	PopulationSize int     `json:"population_size"`
	Generations    int     `json:"generations"`
	MutationRate   float64 `json:"mutation_rate"`
}

// Структура для красивого ответа
type ScheduleItemResponse struct {
	Subject    string   `json:"subject"`
	Instructor string   `json:"instructor"`
	Room       string   `json:"room"`
	Day        string   `json:"day"`
	Time       string   `json:"time"`
	Groups     []string `json:"groups"`
}

// ==========================================
// ЭНДПОИНТ: ГЕНЕТИЧЕСКИЙ АЛГОРИТМ
// ==========================================
func (h *Handler) GenerateScheduleGenetic(c *fiber.Ctx) error {
	// 1. Читаем параметры по умолчанию
	req := GenerateRequest{
		PopulationSize: 200,
		Generations:    200,
		MutationRate:   0.05,
	}

	if err := c.BodyParser(&req); err != nil && len(c.Body()) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON body"})
	}

	// Создаем движок ГА
	engine := genetic.NewEngine(h.DB)
	engine.PopulationSize = req.PopulationSize
	engine.Generations = req.Generations
	engine.MutationRate = req.MutationRate

	startTime := time.Now()
	bestSchedule, err := engine.Run()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	duration := time.Since(startTime)

	// Формируем красивый ответ
	result := h.formatScheduleResponse(bestSchedule, engine.Evaluator)

	return c.JSON(fiber.Map{
		"algorithm":     "Genetic",
		"status":        "success",
		"fitness_score": bestSchedule.Fitness,
		"time_taken_ms": duration.Milliseconds(),
		"parameters":    req,
		"schedule":      result,
	})
}

// ==========================================
// ЭНДПОИНТ: ЖАДНЫЙ АЛГОРИТМ
// ==========================================
func (h *Handler) GenerateScheduleGreedy(c *fiber.Ctx) error {
	engine := greedy.NewEngine(h.DB)

	startTime := time.Now()
	bestSchedule, err := engine.Run()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	duration := time.Since(startTime)

	result := h.formatScheduleResponse(bestSchedule, engine.Evaluator)

	return c.JSON(fiber.Map{
		"algorithm":     "Greedy",
		"status":        "success",
		"fitness_score": bestSchedule.Fitness,
		"time_taken_ms": duration.Milliseconds(),
		"schedule":      result,
	})
}

// Вспомогательная функция (чтобы не дублировать код маппинга)
func (h *Handler) formatScheduleResponse(schedule *algorithm.Schedule, evaluator *algorithm.Evaluator) []ScheduleItemResponse {
	result := make([]ScheduleItemResponse, len(schedule.Assignments))
	for i, assignment := range schedule.Assignments {
		room := evaluator.Context.RoomsMap[assignment.RoomID]
		slot := evaluator.Context.SlotsMap[assignment.SlotID]
		cls := evaluator.Context.ClassesMap[assignment.ClassID]

		var groupNames []string
		for _, g := range cls.Groups {
			groupNames = append(groupNames, g.Name)
		}

		// Обработка отсутствия данных
		var subjName, instName string
		if cls.Subject.Name != "" {
			subjName = cls.Subject.Name
		}
		if cls.Instructor.Name != "" {
			instName = cls.Instructor.Name
		}

		result[i] = ScheduleItemResponse{
			Subject:    subjName,
			Instructor: instName,
			Room:       room.Name,
			Day:        string(slot.Day),
			Time:       fmt.Sprintf("%s - %s", slot.StartTime, slot.EndTime),
			Groups:     groupNames,
		}
	}
	return result
}
