package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"alphacore/internal/models"
)

func LoadIndexConfig(filepath string) (map[string]models.IndexConfig, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var configMap map[string]models.IndexConfig
	if err := json.Unmarshal(bytes, &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return configMap, nil
}

func LoadAppConfig(path string) (*models.AppConfig, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg models.AppConfig
	if err := json.Unmarshal(configFile, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
