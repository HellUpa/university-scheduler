package main

import (
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"scheduler/internal/config"
	"scheduler/internal/database"
	"scheduler/internal/transport"
)

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Cannot load config: %v", err)
	}

	db := database.NewConnection(cfg.GetDSN())
	database.Seed(db)

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173", // Указываем порт Vite (фронтенда)
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	app.Use(logger.New())
	app.Use(recover.New())

	// API ЭНДПОИНТЫ
	handler := transport.NewHandler(db)
	api := app.Group("/api/v1")
	api.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	api.Post("/schedule/generate/genetic", handler.GenerateScheduleGenetic)
	api.Post("/schedule/generate/greedy", handler.GenerateScheduleGreedy)
	api.Get("/schedule/generate/genetic/ws", websocket.New(handler.EvolutionWS))

	log.Printf("Starting server on port %s...", cfg.ServerPort)
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
