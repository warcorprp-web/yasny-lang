//go:build js && wasm
package main

import (
	"bytes"
	"fmt"
	"syscall/js"
	"yasny-lang/lexer"
	"yasny-lang/parser"
	"yasny-lang/interpreter"
)

func runYasny(code string) (result string) {
	// Защита от паник - в WASM критично, иначе runtime умирает
	defer func() {
		if r := recover(); r != nil {
			result = fmt.Sprintf("❌ Внутренняя ошибка: %v\n\n💡 Если это связано с http_получить - в браузере поддержка ограничена.\nПопробуйте локальную установку или используйте mock-данные.", r)
		}
	}()
	
	// Перехватываем вывод
	outputBuffer := &bytes.Buffer{}
	interpreter.OutputWriter = outputBuffer
	
	l := lexer.NewWithFilename(code, "playground.ya")
	p := parser.New(l)
	program := p.ParseProgram()
	
	if len(p.Errors()) > 0 {
		result := "❌ Ошибки парсинга:\n"
		for _, err := range p.Errors() {
			result += "  - " + err + "\n"
		}
		return result
	}
	
	env := interpreter.NewEnvironment()
	evaluated := interpreter.Eval(program, env)
	
	output := outputBuffer.String()
	
	if evaluated != nil && evaluated.Type() == "ERROR" {
		return output + "\n" + evaluated.Inspect()
	}
	
	return output
}

func main() {
	js.Global().Set("yasnyRun", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// Каждый вызов - в собственном recover-обёрнутом контексте
		defer func() {
			recover()
		}()
		
		if len(args) < 1 {
			return "Ошибка: нет кода"
		}
		code := args[0].String()
		return runYasny(code)
	}))
	
	println("Ясный WASM v0.60 загружен!")
	select {}
}
