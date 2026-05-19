//go:build js

package interpreter

// В WASM PostgreSQL не поддерживается (нет сетевого доступа).
func registerPostgresInDB() {}
