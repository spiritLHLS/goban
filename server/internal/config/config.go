package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	Debug              bool
	Username           string
	Password           string
	DBPath             string
	SecretKey          string
	MaxConcurrentTasks int
}

var globalConfig *Config

func LoadConfig() *Config {
	if globalConfig != nil {
		return globalConfig
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	username := os.Getenv("USERNAME")
	if username == "" {
		username = "admin"
	}

	password := os.Getenv("PASSWORD")
	if password == "" {
		password = "admin123"
	}

	debug := os.Getenv("DEBUG")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/goban.db"
	}

	secretKey := os.Getenv("GOBAN_SECRET_KEY")
	if secretKey == "" {
		secretKey = password
	}

	maxConcurrentTasks := getEnvInt("MAX_CONCURRENT_TASKS", 2)
	if maxConcurrentTasks <= 0 {
		maxConcurrentTasks = 2
	}

	globalConfig = &Config{
		Port:               port,
		Debug:              debug == "true",
		Username:           username,
		Password:           password,
		DBPath:             dbPath,
		SecretKey:          secretKey,
		MaxConcurrentTasks: maxConcurrentTasks,
	}

	return globalConfig
}

func GetConfig() *Config {
	if globalConfig == nil {
		return LoadConfig()
	}
	return globalConfig
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
