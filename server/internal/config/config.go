package config

import (
	"os"
)

type Config struct {
	Port     string
	Debug    bool
	Username string
	Password string
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

	globalConfig = &Config{
		Port:     port,
		Debug:    debug == "true",
		Username: username,
		Password: password,
	}

	return globalConfig
}

func GetConfig() *Config {
	if globalConfig == nil {
		return LoadConfig()
	}
	return globalConfig
}
