package interpreter

import (
	"fmt"

	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// === Префиксные операторы ===

func evalPrefixExpression(tok lexer.Token, operator string, right Object) Object {
	switch operator {
	case "не", "!":
		return evalNotOperator(right)
	case "-":
		return evalMinusPrefixOperatorExpression(tok, right)
	default:
		return ErrorWithHint(
			tok,
			fmt.Sprintf("неизвестный оператор: %s%s", operator, right.Type()),
			"Проверьте, что используете правильный унарный оператор (не, !).",
		)
	}
}

func evalNotOperator(right Object) Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(tok lexer.Token, right Object) Object {
	if right.Type() == "INTEGER" {
		value := right.(*Integer).Value
		return &Integer{Value: -value}
	}
	if right.Type() == "FLOAT" {
		value := right.(*Float).Value
		return &Float{Value: -value}
	}
	return ErrorWithHint(
		tok,
		fmt.Sprintf("неизвестный оператор: -%s", right.Type()),
		"Унарный минус можно применять только к числам.",
	)
}

// === Инфиксные операторы ===

func evalInfixExpression(tok lexer.Token, operator string, left, right Object) Object {
	// == и != работают для всех типов через deepEqual.
	if operator == "==" {
		return nativeBoolToBooleanObject(deepEqual(left, right))
	}
	if operator == "!=" {
		return nativeBoolToBooleanObject(!deepEqual(left, right))
	}
	if operator == "и" {
		return nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))
	}
	if operator == "или" {
		return nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right))
	}

	switch {
	case left.Type() == "INTEGER" && right.Type() == "INTEGER":
		return evalIntegerInfixExpression(tok, operator, left, right)
	case left.Type() == "FLOAT" || right.Type() == "FLOAT":
		return evalFloatInfixExpression(tok, operator, left, right)
	case left.Type() == "STRING" && right.Type() == "STRING":
		return evalStringInfixExpression(operator, left, right)
	case operator == "+" && (left.Type() == "STRING" || right.Type() == "STRING"):
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == "ERROR_VALUE" || right.Type() == "ERROR_VALUE":
		return evalStringInfixExpression(operator, left, right)
	case left.Type() != right.Type():
		return ErrorTypeMismatch(tok, left.Type(), operator, right.Type())
	default:
		return ErrorUnknownOperator(tok, left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(tok lexer.Token, operator string, left, right Object) Object {
	leftVal := left.(*Integer).Value
	rightVal := right.(*Integer).Value

	switch operator {
	case "+":
		return &Integer{Value: leftVal + rightVal}
	case "-":
		return &Integer{Value: leftVal - rightVal}
	case "*":
		return &Integer{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return ErrorDivisionByZero(tok)
		}
		// Если деление точное — возвращаем целое, иначе — дробное.
		if leftVal%rightVal == 0 {
			return &Integer{Value: leftVal / rightVal}
		}
		return &Float{Value: float64(leftVal) / float64(rightVal)}
	case "%":
		return &Integer{Value: leftVal % rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return ErrorUnknownOperator(tok, left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(tok lexer.Token, operator string, left, right Object) Object {
	var leftVal, rightVal float64

	if left.Type() == "FLOAT" {
		leftVal = left.(*Float).Value
	} else {
		leftVal = float64(left.(*Integer).Value)
	}

	if right.Type() == "FLOAT" {
		rightVal = right.(*Float).Value
	} else {
		rightVal = float64(right.(*Integer).Value)
	}

	switch operator {
	case "+":
		return &Float{Value: leftVal + rightVal}
	case "-":
		return &Float{Value: leftVal - rightVal}
	case "*":
		return &Float{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return ErrorDivisionByZero(tok)
		}
		return &Float{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return ErrorUnknownOperator(tok, "FLOAT", operator, "FLOAT")
	}
}

func evalStringInfixExpression(operator string, left, right Object) Object {
	if operator != "+" {
		if left.Type() != "STRING" || right.Type() != "STRING" {
			return newError("неизвестный оператор: %s %s %s", left.Type(), operator, right.Type())
		}

		leftVal := left.(*String).Value
		rightVal := right.(*String).Value

		switch operator {
		case "==":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "!=":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return newError("неизвестный оператор: STRING %s STRING", operator)
		}
	}

	// Для + поддерживаем конкатенацию с любыми типами.
	leftVal := stringifyForConcat(left)
	rightVal := stringifyForConcat(right)
	return &String{Value: leftVal + rightVal}
}

// stringifyForConcat возвращает строковое представление значения,
// пригодное для конкатенации через +.
func stringifyForConcat(o Object) string {
	switch o.Type() {
	case "STRING":
		return o.(*String).Value
	case "INTEGER":
		return fmt.Sprintf("%d", o.(*Integer).Value)
	case "FLOAT":
		return fmt.Sprintf("%g", o.(*Float).Value)
	case "ERROR_VALUE":
		return o.(*ErrorValue).Message
	default:
		return o.Inspect()
	}
}

// === Сравнение и тернарный оператор ===

// deepEqual сравнивает объекты с глубокой проверкой структуры.
func deepEqual(a, b Object) bool {
	// NULL == FALSE для совместимости (нет/пусто).
	if (a == NULL && b == FALSE) || (a == FALSE && b == NULL) {
		return true
	}
	// ErrorValue симметрично сравнимо со String.
	if av, ok := a.(*ErrorValue); ok {
		if bv, ok := b.(*ErrorValue); ok {
			return av.Message == bv.Message
		}
		if bv, ok := b.(*String); ok {
			return av.Message == bv.Value
		}
		return false
	}
	if av, ok := a.(*String); ok {
		if bv, ok := b.(*ErrorValue); ok {
			return av.Value == bv.Message
		}
	}
	if a.Type() != b.Type() {
		// 5 == 5.0 поддерживаем
		if (a.Type() == "INTEGER" && b.Type() == "FLOAT") ||
			(a.Type() == "FLOAT" && b.Type() == "INTEGER") {
			af := toFloat(a)
			bf := toFloat(b)
			if af != nil && bf != nil {
				return *af == *bf
			}
		}
		return false
	}
	switch av := a.(type) {
	case *Integer:
		return av.Value == b.(*Integer).Value
	case *Float:
		return av.Value == b.(*Float).Value
	case *String:
		return av.Value == b.(*String).Value
	case *Boolean:
		return av.Value == b.(*Boolean).Value
	case *Array:
		bv := b.(*Array)
		if len(av.Elements) != len(bv.Elements) {
			return false
		}
		for i, el := range av.Elements {
			if !deepEqual(el, bv.Elements[i]) {
				return false
			}
		}
		return true
	case *Hash:
		bv := b.(*Hash)
		if len(av.Pairs) != len(bv.Pairs) {
			return false
		}
		for k, vp := range av.Pairs {
			bp, ok := bv.Pairs[k]
			if !ok {
				return false
			}
			if !deepEqual(vp.Value, bp.Value) {
				return false
			}
		}
		return true
	case *ErrorValue:
		switch bv := b.(type) {
		case *ErrorValue:
			return av.Message == bv.Message
		case *String:
			return av.Message == bv.Value
		}
		return false
	default:
		return a == b
	}
}

// compareObjects — упрощённое сравнение по типу+значению (без глубины).
func compareObjects(a, b Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch a := a.(type) {
	case *Integer:
		return a.Value == b.(*Integer).Value
	case *Float:
		return a.Value == b.(*Float).Value
	case *String:
		return a.Value == b.(*String).Value
	case *Boolean:
		return a.Value == b.(*Boolean).Value
	default:
		return a == b
	}
}

// evalTernaryExpression — условие ? а : б. Тернарник остался как
// краткая форма для совместимости; рекомендуемая запись в идиоматичном
// коде — `если c: a иначе: b`.
func evalTernaryExpression(node *ast.TernaryExpression, env *Environment) Object {
	condition := Eval(node.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(node.Consequence, env)
	}
	return Eval(node.Alternative, env)
}
