package api

import (
	"encoding/json"
	"os"
	"path/filepath"
)

var (
	// 数据目录
	dataDir = "./data"
	// 脚本数据文件路径
	scriptsFilePath = filepath.Join(dataDir, "scripts.json")
	// 备注数据文件路径
	notesFilePath = filepath.Join(dataDir, "notes.json")
	// AuthData 持久化文件路径
	authDataFilePath = filepath.Join(dataDir, "auth_data.txt")
)

// 初始化存储
func InitStorage() error {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	// 确保脚本文件存在
	if _, err := os.Stat(scriptsFilePath); os.IsNotExist(err) {
		// 创建空的脚本文件
		emptyScripts := Scripts{Scripts: []Script{}}
		data, _ := json.MarshalIndent(emptyScripts, "", "  ")
		if err := os.WriteFile(scriptsFilePath, data, 0644); err != nil {
			return err
		}
	}

	// 确保备注文件存在
	if _, err := os.Stat(notesFilePath); os.IsNotExist(err) {
		// 创建空的备注文件
		emptyNotes := Notes{
			CommandNotes: make(map[string]string),
			KeyNotes:     make(map[string]map[string]string),
		}
		data, _ := json.MarshalIndent(emptyNotes, "", "  ")
		if err := os.WriteFile(notesFilePath, data, 0644); err != nil {
			return err
		}
	}

	return nil
}
