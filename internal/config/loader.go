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
		return nil, fmt.Errorf("无法打开配置文件: %v", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	var configMap map[string]models.IndexConfig
	if err := json.Unmarshal(bytes, &configMap); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	return configMap, nil
}