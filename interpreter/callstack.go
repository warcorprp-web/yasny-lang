package interpreter

import (
	"fmt"
	"strings"
	"yasny-lang/lexer"
)

// CallFrame — кадр стека вызовов.
// Создаётся при вызове пользовательской функции и удаляется при возврате.
type CallFrame struct {
	FunctionName string      // имя функции, например "сумма" или "Класс.метод"
	CallToken    lexer.Token // место в коде откуда был вызов
}

// callStack — глобальный стек активных вызовов.
// Не thread-safe, но интерпретатор однопоточный (горутины через async/await
// используют отдельные контексты).
var callStack []CallFrame

// pushFrame добавляет кадр в стек вызовов.
func pushFrame(name string, callTok lexer.Token) {
	callStack = append(callStack, CallFrame{
		FunctionName: name,
		CallToken:    callTok,
	})
}

// popFrame убирает последний кадр из стека.
func popFrame() {
	if len(callStack) > 0 {
		callStack = callStack[:len(callStack)-1]
	}
}

// resetCallStack очищает стек вызовов. Используется при перезапуске
// интерпретатора (например, между тестами или при загрузке модулей).
func resetCallStack() {
	callStack = callStack[:0]
}

// formatCallStack форматирует стек вызовов в читаемый текст.
// Пустой стек не печатается (нет смысла показывать «вызов из main»).
func formatCallStack() string {
	if len(callStack) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n📚 Стек вызовов (от свежего к старому):\n")
	// Идём от верха стека (последний вызов) к низу
	for i := len(callStack) - 1; i >= 0; i-- {
		frame := callStack[i]
		loc := ""
		if frame.CallToken.Filename != "" {
			loc = fmt.Sprintf("%s:%d", frame.CallToken.Filename, frame.CallToken.Line)
		} else if frame.CallToken.Line > 0 {
			loc = fmt.Sprintf("строка %d", frame.CallToken.Line)
		}
		if loc != "" {
			b.WriteString(fmt.Sprintf("   %d. %s — %s\n", len(callStack)-i, frame.FunctionName, loc))
		} else {
			b.WriteString(fmt.Sprintf("   %d. %s\n", len(callStack)-i, frame.FunctionName))
		}
	}
	return b.String()
}
