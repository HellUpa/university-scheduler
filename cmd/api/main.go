package main

import (
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"

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

	// Инициализируем движок шаблонов (указываем папку views и расширение файлов)
	engine := html.New("./views", ".html")

	// Передаем движок в конфигурацию Fiber
	app := fiber.New(fiber.Config{
		AppName: "UCTTP Scheduler API",
		Views:   engine, // <--- ПОДКЛЮЧАЕМ VIEWS
	})

	app.Use(logger.New())
	app.Use(recover.New())

	// ==========================================
	// UI ЭНДПОИНТ (Главная страница)
	// ==========================================
	app.Get("/", func(c *fiber.Ctx) error {
		// Рендерим файл views/index.html
		return c.Render("index", fiber.Map{})
	})

	// API ЭНДПОИНТЫ
	handler := transport.NewHandler(db)
	api := app.Group("/api/v1")
	api.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	api.Post("/schedule/generate/genetic", handler.GenerateScheduleGenetic)
	api.Post("/schedule/generate/greedy", handler.GenerateScheduleGreedy)
	app.Get("/schedule/generate/genetic/ws", websocket.New(handler.EvolutionWS))

	log.Printf("Starting server on port %s...", cfg.ServerPort)
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
