package config

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Postgres   PostgresConfig `mapstructure:"postgres"`
	ServerPort string         `mapstructure:"server_port"`
}

type PostgresConfig struct {
	Host     string `mapstructure:"server"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"db"`
}

func LoadConfig(path string) (config Config, err error) {
	configPath := fetchConfigPath()

	if configPath == "" {
		panic("config path is empty")
	}

	// check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	viper.AddConfigPath(configPath)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.ReadInConfig()
	if err != nil {
		// Если файла .env нет, но есть системные переменные, это нормально для продакшна
		log.Println("No .env file found, relying on environment variables")
	}

	err = viper.Unmarshal(&config)

	// Установим дефолтный порт, если не задан
	if config.ServerPort == "" {
		config.ServerPort = "8000"
	}

	return
}

// GetDSN формирует строку подключения к PostgreSQL
func (c *Config) GetDSN() string {
	return "host=" + c.Postgres.Host +
		" user=" + c.Postgres.User +
		" password=" + c.Postgres.Password +
		" dbname=" + c.Postgres.DBName +
		" port=" + c.Postgres.Port +
		" sslmode=disable TimeZone=UTC"
}

func fetchConfigPath() string {
	res := os.Getenv("CONFIG_PATH")

	if res == "" {
		flag.StringVar(&res, "config", "", "path to config file")
		flag.Parse()
	}

	return res
}
