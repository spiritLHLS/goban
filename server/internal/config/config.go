package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	Debug              bool
	Username           string
	Password           string
	DBPath             string
	SecretKey          string
	AllowedOrigins     []string
	MaxConcurrentTasks int
	DBMaxOpenConns     int
	DBMaxIdleConns     int
	DBConnMaxLifetime  time.Duration
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

	username := firstEnv("GOBAN_USERNAME", "USERNAME")
	if username == "" {
		username = "admin"
	}

	debug := os.Getenv("DEBUG")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/goban.db"
	}

	password := os.Getenv("PASSWORD")
	if password == "" {
		generated, path, err := loadOrCreateSecretFile("GOBAN_PASSWORD_FILE", dbPath, ".goban_admin_password", 24)
		if err != nil {
			log.Printf("[安全] 未设置 PASSWORD，且无法创建管理员密码文件: %v", err)
			generated = mustRandomSecret(24)
		} else {
			log.Printf("[安全] 未设置 PASSWORD，使用管理员密码文件: %s", path)
		}
		password = generated
	}

	secretKey := os.Getenv("GOBAN_SECRET_KEY")
	if secretKey == "" {
		generated, path, err := loadOrCreateSecretFile("GOBAN_SECRET_KEY_FILE", dbPath, ".goban_secret_key", 32)
		if err != nil {
			log.Printf("[安全] 未设置 GOBAN_SECRET_KEY，且无法创建密钥文件: %v", err)
			generated = mustRandomSecret(32)
		} else {
			log.Printf("[安全] 未设置 GOBAN_SECRET_KEY，使用密钥文件: %s", path)
		}
		secretKey = generated
	}

	maxConcurrentTasks := getEnvInt("MAX_CONCURRENT_TASKS", 2)
	if maxConcurrentTasks <= 0 {
		maxConcurrentTasks = 2
	}
	dbMaxOpenConns := getEnvInt("DB_MAX_OPEN_CONNS", 20)
	if dbMaxOpenConns < 1 {
		dbMaxOpenConns = 20
	}
	dbMaxIdleConns := getEnvInt("DB_MAX_IDLE_CONNS", 5)
	if dbMaxIdleConns < 1 {
		dbMaxIdleConns = 5
	}
	if dbMaxIdleConns > dbMaxOpenConns {
		dbMaxIdleConns = dbMaxOpenConns
	}
	dbConnLifetimeSeconds := getEnvInt("DB_CONN_MAX_LIFETIME", 3600)
	if dbConnLifetimeSeconds <= 0 {
		dbConnLifetimeSeconds = 3600
	}

	globalConfig = &Config{
		Port:               port,
		Debug:              debug == "true",
		Username:           username,
		Password:           password,
		DBPath:             dbPath,
		SecretKey:          secretKey,
		AllowedOrigins:     getEnvList("ALLOWED_ORIGINS", defaultAllowedOrigins()),
		MaxConcurrentTasks: maxConcurrentTasks,
		DBMaxOpenConns:     dbMaxOpenConns,
		DBMaxIdleConns:     dbMaxIdleConns,
		DBConnMaxLifetime:  time.Duration(dbConnLifetimeSeconds) * time.Second,
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

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func getEnvList(key string, fallback []string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	if len(values) == 0 {
		return fallback
	}
	return values
}

func defaultAllowedOrigins() []string {
	return []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	}
}

func loadOrCreateSecretFile(envKey, dbPath, filename string, byteLen int) (string, string, error) {
	path := strings.TrimSpace(os.Getenv(envKey))
	if path == "" {
		path = filepath.Join(filepath.Dir(dbPath), filename)
	}
	if raw, err := os.ReadFile(path); err == nil {
		value := strings.TrimSpace(string(raw))
		if value != "" {
			if err := os.Chmod(path, 0600); err != nil {
				return "", path, err
			}
			return value, path, nil
		}
	} else if !os.IsNotExist(err) {
		return "", path, err
	}

	value, err := randomSecret(byteLen)
	if err != nil {
		return "", path, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", path, err
	}
	if err := os.WriteFile(path, []byte(value+"\n"), 0600); err != nil {
		return "", path, fmt.Errorf("写入密钥文件失败: %w", err)
	}
	return value, path, nil
}

func randomSecret(byteLen int) (string, error) {
	raw := make([]byte, byteLen)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func mustRandomSecret(byteLen int) string {
	value, err := randomSecret(byteLen)
	if err != nil {
		panic(err)
	}
	return value
}
