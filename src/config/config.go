package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port     int
	Domain   string
	FileDir  string
}

func LoadConfig() *Config {
	port, _ := strconv.Atoi(getEnvOrDefault("PORT", "4000"))
	
	return &Config{
		Port:     port,
		Domain:   getEnvOrDefault("DOMAIN", "http://localhost:3000"),
		FileDir:  getEnvOrDefault("FILE_DIR", "./uploads"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
