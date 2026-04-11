package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"scheduler/internal/algorithm/genetic"
	"scheduler/internal/config"
	"scheduler/internal/database"
	"strconv"
	"time"
)

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Cannot load config: %v", err)
	}

	db := database.NewConnection(cfg.GetDSN())
	database.Seed(db)

	// 1. Создаем/открываем CSV файл
	file, err := os.Create("/app/benchmarks/results.csv")
	if err != nil {
		log.Fatalf("Cannot create file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Заголовки CSV
	writer.Write([]string{
		"timestamp", "pop_size", "generations", "is_soft_enabled", "run_id", "duration_ms", "penalty",
	})

	// 2. Определяем сетку параметров
	softMutationFlag := []bool{true, false}
	iterations := 30

	log.Println("Starting benchmarks...")

	for pop := 50; pop < 1000; pop += 25 {
		for gen := 100; gen < 3000; gen += 100 {
			for _, soft := range softMutationFlag {
				for i := 1; i <= iterations; i++ {

					log.Printf("Running: Pop=%d, Gen=%.2f, Soft=%v, Run=%d", pop, gen, soft, i)

					start := time.Now()

					eng := genetic.NewEngine(db)

					eng.IsSilent = true

					eng.PopulationSize = pop
					eng.Generations = gen
					eng.IsSoftMutationEnabled = soft

					bestSchedule, _ := eng.Run(nil)

					duration := time.Since(start).Milliseconds()

					// 4. Запись в CSV
					writer.Write([]string{
						time.Now().Format(time.RFC3339),
						strconv.Itoa(pop),
						fmt.Sprintf("%.2f", gen),
						strconv.FormatBool(soft),
						strconv.Itoa(i),
						strconv.FormatInt(duration, 10),
						fmt.Sprintf("%.1f", bestSchedule.InternalPenalty),
					})

					writer.Flush()
				}
			}
		}
	}

	log.Println("Benchmarks finished! Check benchmarks/results.csv")
}
