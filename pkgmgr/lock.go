package pkgmgr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// LockFile — стандартное имя файла блокировки.
const LockFile = "пакет.lock"

// LockEntry описывает один установленный пакет в lock-файле.
type LockEntry struct {
	Name    string `json:"имя"`
	Source  string `json:"источник"` // URL без схемы и без @версия
	Version string `json:"версия"`   // тег/ветка из спецификации
	Commit  string `json:"коммит"`   // точный SHA коммита
}

// Lock — содержимое пакет.lock.
type Lock struct {
	Packages []*LockEntry `json:"пакеты"`
}

// LoadLock читает lock-файл из папки проекта. Если файла нет —
// возвращает пустой Lock без ошибки.
func LoadLock(projectRoot string) (*Lock, error) {
	path := filepath.Join(projectRoot, LockFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Lock{Packages: []*LockEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("не удалось разобрать %s: %w", path, err)
	}
	if l.Packages == nil {
		l.Packages = []*LockEntry{}
	}
	return &l, nil
}

// Save записывает lock-файл с отступами и без HTML-экранирования
// (чтобы кириллица читалась как есть).
func (l *Lock) Save(projectRoot string) error {
	// Сортируем для стабильного diff в git.
	sort.Slice(l.Packages, func(i, j int) bool {
		return l.Packages[i].Name < l.Packages[j].Name
	})

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(l); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectRoot, LockFile), buf.Bytes(), 0644)
}

// Set добавляет или обновляет запись пакета.
func (l *Lock) Set(entry *LockEntry) {
	for i, e := range l.Packages {
		if e.Name == entry.Name {
			l.Packages[i] = entry
			return
		}
	}
	l.Packages = append(l.Packages, entry)
}

// Remove удаляет запись о пакете.
func (l *Lock) Remove(name string) {
	for i, e := range l.Packages {
		if e.Name == name {
			l.Packages = append(l.Packages[:i], l.Packages[i+1:]...)
			return
		}
	}
}

// Get возвращает запись о пакете или nil, если пакета нет.
func (l *Lock) Get(name string) *LockEntry {
	for _, e := range l.Packages {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// CurrentCommit возвращает текущий SHA коммита HEAD в указанной
// рабочей копии git. Используется для записи точной версии в lock.
func CurrentCommit(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse не удался: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
