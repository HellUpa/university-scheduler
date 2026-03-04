package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"scheduler/internal/config"
	"scheduler/internal/database"
	"scheduler/internal/transport"
)

func main() {
	// 1. Загрузка конфигурации
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Cannot load config: %v", err)
	}

	// 2. Подключение к БД
	db := database.NewConnection(cfg.GetDSN())
	log.Println("Database connected and migrated successfully")

	// TODO: Временно! Для тестирования мы можем вызвать функцию Seeder'а здесь
	database.Seed(db)

	// 3. Инициализация Fiber
	app := fiber.New(fiber.Config{
		AppName: "UCTTP Scheduler API",
	})

	// Middleware
	app.Use(logger.New())  // Логирование запросов
	app.Use(recover.New()) // Защита от паники

	// 4. Настройка роутов
	handler := transport.NewHandler(db)

	api := app.Group("/api/v1")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Наша заветная кнопка "Посчитать"
	api.Post("/schedule/generate", handler.GenerateSchedule)

	// 5. Запуск сервера
	log.Printf("Starting server on port %s...", cfg.ServerPort)
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
