package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// Script 定义脚本数据结构
type Script struct {
	ID        string    `json:"id"`        // 脚本ID
	Name      string    `json:"name"`      // 脚本名称
	Content   string    `json:"content"`   // 脚本内容
	Enabled   bool      `json:"enabled"`   // 是否启用
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// Scripts 脚本列表
type Scripts struct {
	Scripts []Script `json:"scripts"`
}

// 保存脚本数据到文件
func saveScripts(scripts Scripts) error {
	data, err := json.MarshalIndent(scripts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(scriptsFilePath, data, 0644)
}

// 从文件加载脚本数据
func loadScripts() (Scripts, error) {
	var scripts Scripts
	scripts.Scripts = make([]Script, 0)

	data, err := os.ReadFile(scriptsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空脚本列表
			return scripts, nil
		}
		return scripts, err
	}

	err = json.Unmarshal(data, &scripts)
	return scripts, err
}

// HandleSaveScript 处理脚本保存请求
func HandleSaveScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持POST请求", http.StatusMethodNotAllowed)
		return
	}

	var script Script
	err := json.NewDecoder(r.Body).Decode(&script)
	if err != nil {
		http.Error(w, "解析请求数据失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 加载现有脚本
	scripts, err := loadScripts()
	if err != nil {
		http.Error(w, "加载脚本失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 检查是否是更新现有脚本
	found := false
	for i, s := range scripts.Scripts {
		if s.ID == script.ID {
			// 更新现有脚本
			script.CreatedAt = s.CreatedAt
			script.UpdatedAt = time.Now()
			scripts.Scripts[i] = script
			found = true
			break
		}
	}

	// 如果是新脚本，添加到列表
	if !found {
		if script.ID == "" {
			// 生成新ID
			script.ID = time.Now().Format("20060102150405")
		}
		script.CreatedAt = time.Now()
		script.UpdatedAt = time.Now()
		scripts.Scripts = append(scripts.Scripts, script)
	}

	// 保存脚本
	err = saveScripts(scripts)
	if err != nil {
		http.Error(w, "保存脚本失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(script)
}

// HandleLoadScripts 处理脚本加载请求
func HandleLoadScripts(w http.ResponseWriter, r *http.Request) {
	scripts, err := loadScripts()
	if err != nil {
		http.Error(w, "加载脚本失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scripts)
}

// HandleDeleteScript 处理脚本删除请求
func HandleDeleteScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持POST请求", http.StatusMethodNotAllowed)
		return
	}

	var requestData struct {
		ID string `json:"id"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "解析请求数据失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 加载现有脚本
	scripts, err := loadScripts()
	if err != nil {
		http.Error(w, "加载脚本失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 查找并删除脚本
	for i, script := range scripts.Scripts {
		if script.ID == requestData.ID {
			// 删除脚本
			scripts.Scripts = append(scripts.Scripts[:i], scripts.Scripts[i+1:]...)
			break
		}
	}

	// 保存更新后的脚本列表
	err = saveScripts(scripts)
	if err != nil {
		http.Error(w, "保存脚本失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true}`))
}
