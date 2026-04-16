package transport

import (
	"fmt"
	"time"

	"github.com/gofiber/contrib/websocket"
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

type MainOptions struct {
	PopulationSize int     `json:"population_size"`
	Generations    int     `json:"generations"`
	MutationRate   float64 `json:"mutation_rate"`
}

type AdditionalOptions struct {
	EliteSize      float64 `json:"elitism"`
	TournamentSize int     `json:"tournament_size"`
	CrossoverRate  float64 `json:"crossover_rate"`
	// Мягкая мутация (без жестких конфликтов)
	IsSoftMutationEnabled bool    `json:"is_soft_mutation_enabled"`
	SoftMutationRate      float64 `json:"soft_mutation_rate"`
	SoftMutationAttempts  int     `json:"soft_mutation_attempts"`
	// Нагрев мутации при стагнации
	HeatStagnantCount int     `json:"heat_stagnant_count"`
	HeatStepScale     float64 `json:"heat_step_scale"`
	// Шоковая терапия, если нагрев не помогает
	ShockStagnantCount    int     `json:"shock_stagnant_count"`
	ShockMutationRate     float64 `json:"shock_mutation_rate"`
	ShockMinRecoveryCount int     `json:"shock_min_recovery_count"`
	ShockRecoveryScale    float64 `json:"shock_recovery_scale"`
}

type RuleOptions struct {
}

// GenerateRequest Структура для приема параметров алгоритма из тела запроса (POST JSON)
type GenerateRequest struct {
	MainOptions       MainOptions       `json:"main_options"`
	AdditionalOptions AdditionalOptions `json:"additional_options"`
}

// ScheduleItemResponse Структура для красивого ответа
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
		MainOptions: MainOptions{
			PopulationSize: 100,
			Generations:    200,
			MutationRate:   0.001,
		},
		AdditionalOptions: AdditionalOptions{
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
		},
	}

	if err := c.BodyParser(&req); err != nil && len(c.Body()) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON body"})
	}

	// Создаем движок ГА
	engine := genetic.NewEngine(h.DB)

	// Настраиваем параметры ГА из запроса
	engine.PopulationSize = req.MainOptions.PopulationSize
	engine.Generations = req.MainOptions.Generations
	engine.BaseMutationRate = req.MainOptions.MutationRate
	engine.EliteSize = req.AdditionalOptions.EliteSize
	engine.TournamentSize = req.AdditionalOptions.TournamentSize
	engine.CrossoverRate = req.AdditionalOptions.CrossoverRate
	engine.IsSoftMutationEnabled = req.AdditionalOptions.IsSoftMutationEnabled
	engine.SoftMutationRate = req.AdditionalOptions.SoftMutationRate
	engine.SoftMutationAttempts = req.AdditionalOptions.SoftMutationAttempts
	engine.HeatStagnantCount = req.AdditionalOptions.HeatStagnantCount
	engine.HeatStepScale = req.AdditionalOptions.HeatStepScale
	engine.ShockStagnantCount = req.AdditionalOptions.ShockStagnantCount
	engine.ShockMutationRate = req.AdditionalOptions.ShockMutationRate
	engine.ShockMinRecoveryCount = req.AdditionalOptions.ShockMinRecoveryCount
	engine.ShockRecoveryScale = req.AdditionalOptions.ShockRecoveryScale

	startTime := time.Now()
	bestSchedule, err := engine.Run(nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// algorithm.DebugConflicts(bestSchedule, engine.Evaluator.Context)

	duration := time.Since(startTime)

	// Формируем красивый ответ
	result := h.formatScheduleResponse(bestSchedule, engine.Evaluator)

	return c.JSON(fiber.Map{
		"algorithm":      "Genetic",
		"status":         "success",
		"fitness_score":  bestSchedule.UserFitness,
		"hard_conflicts": bestSchedule.HardConflicts,
		"time_taken_ms":  duration.Milliseconds(),
		"parameters":     req,
		"schedule":       result,
	})
}

func (h *Handler) EvolutionWS(c *websocket.Conn) {
	// 1. Читаем параметры (клиент пришлет их первым сообщением)
	req := GenerateRequest{
		MainOptions: MainOptions{
			PopulationSize: 100,
			Generations:    200,
			MutationRate:   0.001,
		},
		AdditionalOptions: AdditionalOptions{
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
		},
	}

	if err := c.ReadJSON(&req); err != nil {
		return
	}

	startTime := time.Now()

	engine := genetic.NewEngine(h.DB)
	engine.PopulationSize = req.MainOptions.PopulationSize
	engine.Generations = req.MainOptions.Generations
	engine.BaseMutationRate = req.MainOptions.MutationRate
	engine.EliteSize = req.AdditionalOptions.EliteSize
	engine.TournamentSize = req.AdditionalOptions.TournamentSize
	engine.CrossoverRate = req.AdditionalOptions.CrossoverRate
	engine.IsSoftMutationEnabled = req.AdditionalOptions.IsSoftMutationEnabled
	engine.SoftMutationRate = req.AdditionalOptions.SoftMutationRate
	engine.SoftMutationAttempts = req.AdditionalOptions.SoftMutationAttempts
	engine.HeatStagnantCount = req.AdditionalOptions.HeatStagnantCount
	engine.HeatStepScale = req.AdditionalOptions.HeatStepScale
	engine.ShockStagnantCount = req.AdditionalOptions.ShockStagnantCount
	engine.ShockMutationRate = req.AdditionalOptions.ShockMutationRate
	engine.ShockMinRecoveryCount = req.AdditionalOptions.ShockMinRecoveryCount
	engine.ShockRecoveryScale = req.AdditionalOptions.ShockRecoveryScale

	// 2. Определяем функцию прогресса, которая будет слать данные в сокет
	onProgress := func(gen int, fitness float64, mutRate float64) {
		payload := fiber.Map{
			"type":    "progress",
			"gen":     gen,
			"fitness": fitness,
			"mutRate": mutRate,
		}
		_ = c.WriteJSON(payload) // Отправляем в браузер
	}

	// 3. Запускаем алгоритм
	bestSchedule, _ := engine.Run(onProgress)

	duration := time.Since(startTime)

	// 4. Шлем финальный результат
	_ = c.WriteJSON(fiber.Map{
		"type":           "final",
		"schedule":       h.formatScheduleResponse(bestSchedule, engine.Evaluator),
		"fitness":        bestSchedule.UserFitness,
		"hard_conflicts": bestSchedule.HardConflicts,
		"time_taken_ms":  duration.Milliseconds(),
	})
}

// ==========================================
// ЭНДПОИНТ: ЖАДНЫЙ АЛГОРИТМ
// ==========================================
func (h *Handler) GenerateScheduleGreedy(c *fiber.Ctx) error {
	engine := greedy.NewEngine(h.DB)

	startTime := time.Now()
	bestSchedule, err := engine.Run(false)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	duration := time.Since(startTime)

	result := h.formatScheduleResponse(bestSchedule, engine.Evaluator)

	return c.JSON(fiber.Map{
		"algorithm":      "Greedy",
		"status":         "success",
		"fitness_score":  bestSchedule.UserFitness,
		"hard_conflicts": bestSchedule.HardConflicts,
		"time_taken_ms":  duration.Milliseconds(),
		"schedule":       result,
	})
}

// Вспомогательная функция (чтобы не дублировать код маппинга)
func (h *Handler) formatScheduleResponse(schedule *algorithm.Schedule, evaluator *algorithm.Evaluator) []ScheduleItemResponse {
	result := make([]ScheduleItemResponse, len(schedule.Assignments))
	for i, assignment := range schedule.Assignments {
		room := evaluator.Data.RoomsMap[assignment.RoomID]
		slot := evaluator.Data.SlotsMap[assignment.SlotID]
		cls := evaluator.Data.ClassesMap[assignment.ClassID]

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
