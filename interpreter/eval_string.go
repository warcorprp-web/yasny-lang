package interpreter

import (
	"strings"

	"yasny-lang/ast"
	"yasny-lang/lexer"
	"yasny-lang/parser"
)

// evalInterpolatedString вычисляет шаблонную строку с {выражениями}.
func evalInterpolatedString(template string, env *Environment) Object {
	var result strings.Builder

	for i := 0; i < len(template); i++ {
		if template[i] == '{' {
			// Находим закрывающую скобку с учётом вложенности.
			j := i + 1
			depth := 1
			for j < len(template) && depth > 0 {
				if template[j] == '{' {
					depth++
				} else if template[j] == '}' {
					depth--
				}
				j++
			}

			if depth == 0 {
				exprStr := template[i+1 : j-1]

				if len(exprStr) == 0 {
					return newError("пустое выражение в интерполяции")
				}

				l := lexer.New(exprStr)
				p := parser.New(l)
				program := p.ParseProgram()

				if len(p.Errors()) > 0 {
					return newError("ошибка парсинга в интерполяции '{%s}': %s", exprStr, p.Errors()[0])
				}

				if len(program.Statements) == 0 {
					return newError("пустое выражение в интерполяции")
				}

				stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
				if !ok {
					return newError("ожидалось выражение в интерполяции, получено: %T", program.Statements[0])
				}

				val := Eval(stmt.Expression, env)
				if isError(val) {
					return val
				}

				result.WriteString(val.Inspect())
				i = j - 1
				continue
			}
		}

		result.WriteByte(template[i])
	}

	return &String{Value: result.String()}
}
