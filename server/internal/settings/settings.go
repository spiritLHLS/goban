package settings

import (
	"strconv"

	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
)

type Values map[string]string

func All() (Values, error) {
	db := database.GetDB()
	var rows []models.AppSetting
	if err := db.Order("key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	values := Values{}
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	return values, nil
}

func Save(values Values) error {
	db := database.GetDB()
	for key, value := range values {
		row := models.AppSetting{Key: key}
		if err := db.FirstOrCreate(&row, models.AppSetting{Key: key}).Error; err != nil {
			return err
		}
		if err := db.Model(&row).Update("value", value).Error; err != nil {
			return err
		}
	}
	return nil
}

func Get(key, fallback string) string {
	db := database.GetDB()
	var row models.AppSetting
	if err := db.First(&row, "key = ?", key).Error; err != nil || row.Value == "" {
		return fallback
	}
	return row.Value
}

func GetInt(key string, fallback int) int {
	value := Get(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func GetBool(key string, fallback bool) bool {
	value := Get(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
