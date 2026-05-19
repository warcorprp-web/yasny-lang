// Пакет pkgmgr реализует управление пакетами для языка Ясный.
//
// Файл пакет.json описывает проект (имя, версия, точка входа,
// зависимости). Папка пакеты/ хранит установленные зависимости —
// каждый пакет в своей подпапке. На старте — только git/GitHub,
// без центрального реестра.
package pkgmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestFile — стандартное имя файла-манифеста проекта.
const ManifestFile = "пакет.json"

// PackagesDir — стандартное имя папки с установленными пакетами.
const PackagesDir = "пакеты"

// DefaultEntryPoint — стандартное имя точки входа для нового проекта.
const DefaultEntryPoint = "главный.ya"

// Manifest описывает проект на Ясном или библиотеку.
//
// Поля сохраняются в JSON с русскими ключами, чтобы манифест
// читался естественно для русскоязычного автора.
type Manifest struct {
	Name        string            `json:"имя"`
	Version     string            `json:"версия"`
	Description string            `json:"описание,omitempty"`
	Author      string            `json:"автор,omitempty"`
	License     string            `json:"лицензия,omitempty"`
	Entry       string            `json:"точка_входа,omitempty"`
	Deps        map[string]string `json:"зависимости,omitempty"`
}

// Load читает манифест из указанного файла.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("не удалось разобрать %s: %w", path, err)
	}
	if m.Deps == nil {
		m.Deps = map[string]string{}
	}
	return &m, nil
}

// Save записывает манифест в указанный файл с отступами по 2 пробела.
func (m *Manifest) Save(path string) error {
	return os.WriteFile(path, encodeJSON(m), 0644)
}

// encodeJSON сериализует манифест без экранирования кириллицы.
func encodeJSON(m *Manifest) []byte {
	buf := &strings.Builder{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(m)
	// Encode добавляет завершающий \n — это нормально для файла.
	return []byte(buf.String())
}

// FindRoot ищет корень проекта (папку с пакет.json) поднимаясь
// вверх от указанной стартовой папки. Возвращает абсолютный путь
// к корню или ошибку, если манифест не найден.
func FindRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	dir := abs
	for {
		candidate := filepath.Join(dir, ManifestFile)
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("не найден %s — это не проект Ясного", ManifestFile)
		}
		dir = parent
	}
}

// LoadFromCwd ищет манифест начиная с текущего каталога и поднимаясь
// выше, потом загружает его. Возвращает корень проекта и манифест.
func LoadFromCwd() (string, *Manifest, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}
	root, err := FindRoot(cwd)
	if err != nil {
		return "", nil, err
	}
	m, err := Load(filepath.Join(root, ManifestFile))
	if err != nil {
		return "", nil, err
	}
	return root, m, nil
}
