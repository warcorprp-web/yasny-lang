package pkgmgr

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ParseDependency разбирает строку зависимости вида
//   github.com/user/repo
//   github.com/user/repo@v1.0.0
//   github.com/user/repo@main
// Возвращает URL для git и версию (тег/ветку, может быть пустой).
func ParseDependency(spec string) (url, version string) {
	if i := strings.LastIndex(spec, "@"); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, ""
}

// PackageNameFromSpec извлекает короткое имя пакета из спецификации:
// github.com/user/имя_пакета[@версия] -> имя_пакета.
func PackageNameFromSpec(spec string) string {
	url, _ := ParseDependency(spec)
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")
	return name
}

// gitCloneURL добавляет схему https:// если её нет, чтобы git понял URL.
func gitCloneURL(spec string) string {
	url, _ := ParseDependency(spec)
	if strings.Contains(url, "://") {
		return url
	}
	return "https://" + url
}

// Install устанавливает один пакет в папку пакеты/имя.
// Если пакет уже установлен, выполняет git fetch + checkout вместо
// клонирования. Возвращает короткое имя пакета.
func Install(projectRoot, spec string) (string, error) {
	url := gitCloneURL(spec)
	_, version := ParseDependency(spec)
	name := PackageNameFromSpec(spec)
	if name == "" {
		return "", fmt.Errorf("не удалось определить имя пакета из %q", spec)
	}

	pkgDir := filepath.Join(projectRoot, PackagesDir, name)

	if _, err := os.Stat(pkgDir); err == nil {
		// Уже установлен — обновим до нужной версии (если задана).
		if version != "" {
			if err := runGit(pkgDir, "fetch", "--all", "--tags"); err != nil {
				return name, err
			}
			if err := runGit(pkgDir, "checkout", version); err != nil {
				return name, fmt.Errorf("не удалось переключиться на %s: %w", version, err)
			}
		}
		return name, nil
	}

	if err := os.MkdirAll(filepath.Join(projectRoot, PackagesDir), 0755); err != nil {
		return "", err
	}

	args := []string{"clone"}
	if version != "" {
		// --branch принимает и теги, и ветки.
		args = append(args, "--branch", version, "--depth", "1")
	} else {
		args = append(args, "--depth", "1")
	}
	args = append(args, url, pkgDir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone не удался: %w", err)
	}

	return name, nil
}

// InstallAll ставит все зависимости из манифеста.
func InstallAll(projectRoot string, m *Manifest) error {
	if len(m.Deps) == 0 {
		fmt.Println("В манифесте нет зависимостей.")
		return nil
	}
	for alias, spec := range m.Deps {
		fmt.Printf("→ %s (%s)\n", alias, spec)
		if _, err := Install(projectRoot, spec); err != nil {
			return fmt.Errorf("не удалось установить %s: %w", alias, err)
		}
	}
	return nil
}

// Uninstall удаляет пакет из папки пакеты/ и из манифеста.
func Uninstall(projectRoot string, m *Manifest, alias string) error {
	if _, ok := m.Deps[alias]; !ok {
		// Алиас может совпадать с именем пакета — попробуем найти по
		// имени папки.
		for a, spec := range m.Deps {
			if PackageNameFromSpec(spec) == alias {
				alias = a
				goto found
			}
		}
		return fmt.Errorf("пакет %q не найден в манифесте", alias)
	}
found:

	spec := m.Deps[alias]
	name := PackageNameFromSpec(spec)
	pkgDir := filepath.Join(projectRoot, PackagesDir, name)
	if err := os.RemoveAll(pkgDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(m.Deps, alias)
	return nil
}

// runGit выполняет git-команду в указанной папке.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
