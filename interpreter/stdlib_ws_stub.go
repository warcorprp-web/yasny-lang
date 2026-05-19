//go:build js

package interpreter

// В WASM WebSocket не поддерживается через gorilla/websocket.
func registerWebSocketModule() {}
