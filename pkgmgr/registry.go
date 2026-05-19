package pkgmgr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RegistryURL — URL официального реестра коротких имён пакетов.
// Файл загружается напрямую через raw.githubusercontent.com.
const RegistryURL = "https://raw.githubusercontent.com/warcorprp-web/yasny-registry/main/registry.json"

// Registry представляет один пакет в реестре.
type RegistryEntry struct {
	URL         string `json:"url"`
	Description string `json:"описание"`
}

// Registry — содержимое файла registry.json реестра.
type Registry struct {
	Version  int                       `json:"версия_реестра"`
	Packages map[string]*RegistryEntry `json:"пакеты"`
}

// FetchRegistry загружает реестр коротких имён с GitHub.
func FetchRegistry() (*Registry, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(RegistryURL)
	if err != nil {
		return nil, fmt.Errorf("не удалось скачать реестр: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("реестр вернул HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать ответ реестра: %w", err)
	}

	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("реестр испорчен: %w", err)
	}
	return &r, nil
}

// ResolveName ищет короткое имя в реестре. Возвращает URL пакета
// или пустую строку, если имя не найдено.
func (r *Registry) ResolveName(name string) string {
	if r.Packages == nil {
		return ""
	}
	if entry, ok := r.Packages[name]; ok {
		return entry.URL
	}
	return ""
}

// IsShortName определяет, выглядит ли строка как короткое имя
// (не URL): нет слешей, точек, не начинается с http.
func IsShortName(s string) bool {
	// Если есть @ — отделяем версию.
	if i := strings.LastIndex(s, "@"); i > 0 {
		s = s[:i]
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return false
	}
	if strings.Contains(s, "/") {
		return false
	}
	if strings.Contains(s, ".") {
		return false
	}
	return s != ""
}
