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

// InstallAs устанавливает пакет под указанным именем. Это нужно
// когда пользователь подключает пакет по короткому имени (матем),
// а реальный URL — другой (github.com/.../matematika_plus). Папка
// пакеты/матем/ должна совпадать с импортом 'из "матем"'.
//
// Если installAs пуст — имя берётся из URL (как раньше).
func InstallAs(projectRoot, spec, installAs string) (name string, commit string, err error) {
	url := gitCloneURL(spec)
	_, version := ParseDependency(spec)
	if installAs == "" {
		installAs = PackageNameFromSpec(spec)
	}
	if installAs == "" {
		return "", "", fmt.Errorf("не удалось определить имя пакета из %q", spec)
	}

	pkgDir := filepath.Join(projectRoot, PackagesDir, installAs)

	if _, err := os.Stat(pkgDir); err == nil {
		// Уже установлен — обновим до нужной версии (если задана).
		if version != "" {
			if err := runGit(pkgDir, "fetch", "--all", "--tags"); err != nil {
				return installAs, "", err
			}
			if err := runGit(pkgDir, "checkout", version); err != nil {
				return installAs, "", fmt.Errorf("не удалось переключиться на %s: %w", version, err)
			}
		}
		commit, _ = CurrentCommit(pkgDir)
		return installAs, commit, nil
	}

	if err := os.MkdirAll(filepath.Join(projectRoot, PackagesDir), 0755); err != nil {
		return "", "", err
	}

	args := []string{"clone"}
	if version != "" && !isSHA(version) {
		args = append(args, "--branch", version, "--depth", "1")
	} else {
		args = append(args, "--depth", "50")
	}
	args = append(args, url, pkgDir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("git clone не удался: %w", err)
	}

	if version != "" && isSHA(version) {
		if err := runGit(pkgDir, "checkout", version); err != nil {
			_ = runGit(pkgDir, "fetch", "--unshallow")
			if err := runGit(pkgDir, "checkout", version); err != nil {
				return "", "", fmt.Errorf("не удалось перейти на коммит %s: %w", version, err)
			}
		}
	}

	commit, _ = CurrentCommit(pkgDir)
	return installAs, commit, nil
}

// Install — обёртка над InstallAs, имя берётся из URL.
func Install(projectRoot, spec string) (name string, commit string, err error) {
	return InstallAs(projectRoot, spec, "")
}

// isSHA проверяет, выглядит ли строка как SHA-1 коммита (40 hex).
func isSHA(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// InstallAll ставит все зависимости из манифеста с использованием
// lock-файла, если он есть. Lock-файл фиксирует точные коммиты для
// воспроизводимости установки.
//
// Каждая зависимость alias → spec ставится в папку пакеты/alias/,
// чтобы импорт 'из "alias"' нашёл её.
func InstallAll(projectRoot string, m *Manifest) error {
	if len(m.Deps) == 0 {
		fmt.Println("В манифесте нет зависимостей.")
		return nil
	}

	lock, _ := LoadLock(projectRoot)

	for alias, spec := range m.Deps {
		fmt.Printf("→ %s (%s)\n", alias, spec)

		// Нормализуем spec: если это короткое имя — резолвим через реестр.
		realSpec := spec
		if IsShortName(spec) {
			r, err := FetchRegistry()
			if err != nil {
				return fmt.Errorf("реестр недоступен: %w", err)
			}
			name, version := ParseDependency(spec)
			url := r.ResolveName(name)
			if url == "" {
				return fmt.Errorf("имя '%s' не найдено в реестре", name)
			}
			if version != "" {
				realSpec = url + "@" + version
			} else {
				realSpec = url
			}
		}

		// Если в lock есть точный коммит — берём его.
		installSpec := realSpec
		if entry := lock.Get(alias); entry != nil && entry.Commit != "" {
			source, _ := ParseDependency(realSpec)
			if source == entry.Source {
				installSpec = entry.Source + "@" + entry.Commit
			}
		}

		_, commit, err := InstallAs(projectRoot, installSpec, alias)
		if err != nil {
			return fmt.Errorf("не удалось установить %s: %w", alias, err)
		}

		source, version := ParseDependency(realSpec)
		lock.Set(&LockEntry{
			Name:    alias,
			Source:  source,
			Version: version,
			Commit:  commit,
		})
	}

	if err := lock.Save(projectRoot); err != nil {
		return fmt.Errorf("не удалось записать %s: %w", LockFile, err)
	}
	return nil
}

// Uninstall удаляет пакет из папки пакеты/, из манифеста и из lock-файла.
//
// alias — имя в манифесте (т.е. имя папки в пакеты/, под которым
// пакет был установлен).
func Uninstall(projectRoot string, m *Manifest, alias string) error {
	if _, ok := m.Deps[alias]; !ok {
		return fmt.Errorf("пакет %q не найден в манифесте", alias)
	}

	pkgDir := filepath.Join(projectRoot, PackagesDir, alias)
	if err := os.RemoveAll(pkgDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(m.Deps, alias)

	// Обновляем lock-файл.
	if lock, err := LoadLock(projectRoot); err == nil {
		lock.Remove(alias)
		_ = lock.Save(projectRoot)
	}
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
