package transport

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"scheduler/internal/algorithm/genetic"
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
	Subject    string `json:"subject"`
	Instructor string `json:"instructor"`
	Room       string `json:"room"`
	Day        string `json:"day"`
	Time       string `json:"time"`
	Groups     string `json:"groups"`
}

func (h *Handler) GenerateSchedule(c *fiber.Ctx) error {
	// 1. Читаем параметры по умолчанию
	req := GenerateRequest{
		PopulationSize: 100,
		Generations:    50,
		MutationRate:   0.05,
	}

	// 2. Если пользователь прислал JSON, перезаписываем параметры
	if err := c.BodyParser(&req); err != nil && len(c.Body()) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON body",
		})
	}

	// Создаем движок алгоритма
	engine := genetic.NewEngine(h.DB)
	engine.PopulationSize = req.PopulationSize
	engine.Generations = req.Generations
	engine.MutationRate = req.MutationRate

	startTime := time.Now()

	// Запуск алгоритма
	bestIndividual, err := engine.Run()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate schedule: " + err.Error(),
		})
	}

	duration := time.Since(startTime)

	// 3. Формируем "красивый" ответ (Hydration)
	// У engine есть доступ ко всем загруженным данным, используем их для маппинга ID -> Название
	result := make([]ScheduleItemResponse, len(bestIndividual.Genes))

	for i, gene := range bestIndividual.Genes {
		room := engine.Evaluator.RoomsMap[gene.RoomID]
		slot := engine.Evaluator.SlotsMap[gene.SlotID]
		cls := engine.ClassesMap[gene.ClassID]

		var groupNames []string
		for _, g := range cls.Groups {
			groupNames = append(groupNames, g.Name)
		}
		groupsStr := strings.Join(groupNames, ", ")

		result[i] = ScheduleItemResponse{
			Subject:    cls.Subject.Name,
			Instructor: cls.Instructor.Name,
			Room:       room.Name,
			Day:        string(slot.Day),
			Time:       fmt.Sprintf("%s - %s", slot.StartTime, slot.EndTime),
			Groups:     groupsStr,
		}
	}

	return c.JSON(fiber.Map{
		"status":        "success",
		"fitness_score": bestIndividual.Fitness,
		"time_taken_ms": duration.Milliseconds(),
		"parameters":    req, // Возвращаем параметры, чтобы видеть, с чем отработал алгоритм
		"schedule":      result,
	})
}
