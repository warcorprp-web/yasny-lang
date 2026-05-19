//go:build js

package interpreter

// В WASM базы данных не поддерживаются.
func registerDBModule() {}
