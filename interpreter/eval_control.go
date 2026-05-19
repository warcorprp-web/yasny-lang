package interpreter

import (
	"yasny-lang/ast"
)

// === Условные выражения ===

func evalIfExpression(ie *ast.IfExpression, env *Environment) Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalMatchExpression(me *ast.MatchExpression, env *Environment) Object {
	value := Eval(me.Value, env)
	if isError(value) {
		return value
	}

	for _, matchCase := range me.Cases {
		// Default case (иначе)
		if matchCase.Pattern == nil {
			return Eval(matchCase.Result, env)
		}

		pattern := Eval(matchCase.Pattern, env)
		if isError(pattern) {
			return pattern
		}

		// Если паттерн булев, а значение — нет, считаем это как
		// условие-страж: совпадение оценка ... когда оценка >= 90: ...
		if pattern.Type() == "BOOLEAN" && value.Type() != "BOOLEAN" {
			if pattern.(*Boolean).Value {
				return Eval(matchCase.Result, env)
			}
			continue
		}

		if compareObjects(value, pattern) {
			return Eval(matchCase.Result, env)
		}
	}

	return FALSE
}

// === Циклы ===

func evalForExpression(fe *ast.ForExpression, env *Environment) Object {
	from := Eval(fe.From, env)
	if isError(from) {
		return from
	}
	to := Eval(fe.To, env)
	if isError(to) {
		return to
	}

	if from.Type() != "INTEGER" || to.Type() != "INTEGER" {
		return newError("для цикла требуются целые числа")
	}

	fromVal := from.(*Integer).Value
	toVal := to.(*Integer).Value

	// Шаг по умолчанию: 1 если идём вверх, -1 если вниз.
	var stepVal int64 = 1
	if fromVal > toVal {
		stepVal = -1
	}

	// Если задан явный шаг: 'по N'.
	if fe.Step != nil {
		step := Eval(fe.Step, env)
		if isError(step) {
			return step
		}
		if step.Type() != "INTEGER" {
			return newError("шаг цикла должен быть целым числом")
		}
		stepVal = step.(*Integer).Value
		if stepVal == 0 {
			return newError("шаг цикла не может быть нулём")
		}
	}

	var result Object = NULL
	cond := func(i int64) bool {
		if stepVal > 0 {
			return i <= toVal
		}
		return i >= toVal
	}
	for i := fromVal; cond(i); i += stepVal {
		env.Set(fe.Variable.Value, NewInteger(i))
		result = Eval(fe.Body, env)
		if isError(result) {
			return result
		}
		if result.Type() == "RETURN_VALUE" {
			return result
		}
		if result.Type() == "BREAK" {
			return NULL
		}
		if result.Type() == "CONTINUE" {
			continue
		}
	}

	return result
}

func evalForInExpression(fie *ast.ForInExpression, env *Environment) Object {
	iterable := Eval(fie.Iterable, env)
	if isError(iterable) {
		return iterable
	}

	var result Object = NULL

	switch iter := iterable.(type) {
	case *Array:
		for idx, element := range iter.Elements {
			if fie.Index != nil {
				env.Set(fie.Index.Value, NewInteger(int64(idx)))
			}
			env.Set(fie.Variable.Value, element)
			result = Eval(fie.Body, env)
			if isError(result) {
				return result
			}
			if result.Type() == "RETURN_VALUE" {
				return result
			}
			if result.Type() == "BREAK" {
				return NULL
			}
			if result.Type() == "CONTINUE" {
				continue
			}
		}
	case *Hash:
		for idx, pair := range iter.orderedPairs() {
			if fie.Index != nil {
				env.Set(fie.Index.Value, NewInteger(int64(idx)))
			}
			env.Set(fie.Variable.Value, pair.Value)
			result = Eval(fie.Body, env)
			if isError(result) {
				return result
			}
			if result.Type() == "RETURN_VALUE" {
				return result
			}
			if result.Type() == "BREAK" {
				return NULL
			}
			if result.Type() == "CONTINUE" {
				continue
			}
		}
	case *String:
		for idx, ch := range iter.Value {
			if fie.Index != nil {
				env.Set(fie.Index.Value, NewInteger(int64(idx)))
			}
			env.Set(fie.Variable.Value, &String{Value: string(ch)})
			result = Eval(fie.Body, env)
			if isError(result) {
				return result
			}
			if result.Type() == "RETURN_VALUE" {
				return result
			}
			if result.Type() == "BREAK" {
				return NULL
			}
			if result.Type() == "CONTINUE" {
				continue
			}
		}
	case *Generator:
		idx := 0
		for {
			val, ok := iter.Next()
			if !ok {
				break
			}
			if fie.Index != nil {
				env.Set(fie.Index.Value, NewInteger(int64(idx)))
			}
			env.Set(fie.Variable.Value, val)
			result = Eval(fie.Body, env)
			if isError(result) {
				iter.Close()
				return result
			}
			if result.Type() == "RETURN_VALUE" {
				iter.Close()
				return result
			}
			if result.Type() == "BREAK" {
				iter.Close()
				return NULL
			}
			if result.Type() == "CONTINUE" {
				idx++
				continue
			}
			idx++
		}
	default:
		return newError("итерация не поддерживается для типа %s", iterable.Type())
	}

	return result
}

func evalWhileExpression(we *ast.WhileExpression, env *Environment) Object {
	var result Object = NULL

	for {
		condition := Eval(we.Condition, env)
		if isError(condition) {
			return condition
		}

		if !isTruthy(condition) {
			break
		}

		result = Eval(we.Body, env)
		if isError(result) {
			return result
		}
		if result.Type() == "RETURN_VALUE" {
			return result
		}
		if result.Type() == "BREAK" {
			return NULL
		}
		if result.Type() == "CONTINUE" {
			continue
		}
	}

	return result
}

// === Диапазоны и comprehensions ===

func evalRangeExpression(node *ast.RangeExpression, env *Environment) Object {
	start := Eval(node.Start, env)
	if isError(start) {
		return start
	}

	end := Eval(node.End, env)
	if isError(end) {
		return end
	}

	if start.Type() != "INTEGER" || end.Type() != "INTEGER" {
		return newErrorWithToken(node.Token, "диапазон требует целые числа")
	}

	startVal := start.(*Integer).Value
	endVal := end.(*Integer).Value

	elements := []Object{}
	for i := startVal; i < endVal; i++ {
		elements = append(elements, NewInteger(i))
	}

	return &Array{Elements: elements}
}

func evalArrayComprehension(node *ast.ArrayComprehension, env *Environment) Object {
	iterable := Eval(node.Iterable, env)
	if isError(iterable) {
		return iterable
	}

	var elements []Object

	switch iter := iterable.(type) {
	case *Array:
		for _, item := range iter.Elements {
			env.Set(node.Variable.Value, item)

			if node.Condition != nil {
				condition := Eval(node.Condition, env)
				if isError(condition) {
					return condition
				}
				if !isTruthy(condition) {
					continue
				}
			}

			element := Eval(node.Element, env)
			if isError(element) {
				return element
			}
			elements = append(elements, element)
		}
	default:
		return newErrorWithToken(node.Token, "comprehension требует итерируемый объект")
	}

	return &Array{Elements: elements}
}

// === Обработка ошибок ===

func evalTryExpression(te *ast.TryExpression, env *Environment) Object {
	result := evalBlockStatement(te.Body, env)

	if isError(result) && te.CatchBody != nil {
		catchEnv := NewEnclosedEnvironment(env)

		if te.CatchVar != nil {
			// Преобразуем Error в ErrorValue, чтобы он не
			// продолжал распространяться как ошибка.
			errorValue := &ErrorValue{Message: result.(*Error).Message}
			catchEnv.Set(te.CatchVar.Value, errorValue)
		}

		result = evalBlockStatement(te.CatchBody, catchEnv)
	}

	// finally выполняется в любом случае; ошибка или return из
	// finally имеют приоритет над результатом try/catch.
	if te.FinallyBody != nil {
		finallyResult := evalBlockStatement(te.FinallyBody, env)
		if finallyResult != nil && (finallyResult.Type() == "ERROR" || finallyResult.Type() == "RETURN_VALUE") {
			return finallyResult
		}
	}

	return result
}
