package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
)

// Init создаёт пакет.json и стартовый главный.ya в указанной
// папке. Если папка не существует — создаёт её. Если манифест уже
// есть — отказывается, чтобы не затереть.
func Init(dir, name string) error {
	if name == "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		name = filepath.Base(abs)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	manifestPath := filepath.Join(dir, ManifestFile)
	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("в %s уже есть %s — инициализация отменена", dir, ManifestFile)
	}

	m := &Manifest{
		Name:        name,
		Version:     "0.1.0",
		Description: "",
		Author:      "",
		License:     "",
		Entry:       DefaultEntryPoint,
		Deps:        map[string]string{},
	}
	if err := m.Save(manifestPath); err != nil {
		return err
	}

	entryPath := filepath.Join(dir, DefaultEntryPoint)
	if _, err := os.Stat(entryPath); os.IsNotExist(err) {
		stub := fmt.Sprintf("# %s — точка входа проекта.\n\nвывод(\"Привет из проекта %s!\")\n", DefaultEntryPoint, name)
		if err := os.WriteFile(entryPath, []byte(stub), 0644); err != nil {
			return err
		}
	}

	gitignorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		_ = os.WriteFile(gitignorePath, []byte(PackagesDir+"/\n"), 0644)
	}

	return nil
}
