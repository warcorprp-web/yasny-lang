package interpreter

import (
	"strings"

	"yasny-lang/ast"
	"yasny-lang/lexer"
	"yasny-lang/parser"
)

// evalInterpolatedString вычисляет шаблонную строку с {выражениями}.
//
// Если содержимое скобок не парсится как валидное выражение
// (например, `{{var}}` для шаблонизатора, CSS `{color: red}`,
// JSON `{"a": 1}`) — фигурные скобки и их содержимое остаются
// как литерал. Это позволяет писать HTML/CSS/шаблонные строки
// без экранирования.
//
// Опечатки в именах переменных по-прежнему ловятся: они
// успешно парсятся, но падают на этапе вычисления.
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

				// Пустое выражение `{}` — оставляем как литерал.
				if len(exprStr) == 0 {
					result.WriteString(template[i:j])
					i = j - 1
					continue
				}

				l := lexer.New(exprStr)
				p := parser.New(l)
				program := p.ParseProgram()

				// Не парсится как выражение — литерал.
				// Это покрывает `{{var}}`, CSS `{color: red}`,
				// JSON `{"a": 1}` и т.п.
				if len(p.Errors()) > 0 || len(program.Statements) == 0 {
					result.WriteString(template[i:j])
					i = j - 1
					continue
				}

				stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
				if !ok {
					result.WriteString(template[i:j])
					i = j - 1
					continue
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
