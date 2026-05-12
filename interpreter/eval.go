package interpreter

import (
	"fmt"
	"os"
	"yasny-lang/ast"
	"yasny-lang/lexer"
	"yasny-lang/parser"
	"strings"
)

// Глобальный контекст для передачи токена в builtin функции
var currentCallToken lexer.Token

func init() {
	ApplyFunctionCallback = applyFunctionFromBuiltin
}

// Eval выполняет AST узел
func Eval(node ast.Node, env *Environment) Object {
	switch node := node.(type) {

	// Программа
	case *ast.Program:
		return evalProgram(node.Statements, env)

	// Statements
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)

	case *ast.LetStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.SetImmutable(node.Name.Value, val)
		return val

	case *ast.DestructuringStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		
		// Деструктуризация массива: конст [a, b, c] = массив
		if arrayPattern, ok := node.Pattern.(*ast.ArrayLiteral); ok {
			if val.Type() != "ARRAY" {
				return newErrorWithToken(node.Token, "деструктуризация массива требует ARRAY, получено %s", val.Type())
			}
			
			arr := val.(*Array)
			restIndex := -1
			
			// Ищем rest оператор
			for i, elem := range arrayPattern.Elements {
				if spreadExpr, ok := elem.(*ast.SpreadExpression); ok {
					restIndex = i
					// Rest должен содержать идентификатор
					if ident, ok := spreadExpr.Value.(*ast.Identifier); ok {
						// Собираем остальные элементы
						var restElements []Object
						for j := i; j < len(arr.Elements); j++ {
							restElements = append(restElements, arr.Elements[j])
						}
						env.SetImmutable(ident.Value, &Array{Elements: restElements})
					} else {
						return newErrorWithToken(node.Token, "rest оператор должен содержать идентификатор")
					}
					break
				}
			}
			
			// Обрабатываем обычные элементы
			for i, elem := range arrayPattern.Elements {
				if i == restIndex {
					break // Пропускаем rest
				}
				
				if ident, ok := elem.(*ast.Identifier); ok {
					if i < len(arr.Elements) {
						env.SetImmutable(ident.Value, arr.Elements[i])
					} else {
						env.SetImmutable(ident.Value, NULL)
					}
				} else if _, ok := elem.(*ast.SpreadExpression); !ok {
					return newErrorWithToken(node.Token, "паттерн деструктуризации должен содержать только идентификаторы")
				}
			}
			return val
		}
		
		// Деструктуризация объекта: конст {x, y} = объект
		if hashPattern, ok := node.Pattern.(*ast.HashLiteral); ok {
			if val.Type() != "HASH" {
				return newErrorWithToken(node.Token, "деструктуризация объекта требует HASH, получено %s", val.Type())
			}
			
			hash := val.(*Hash)
			for keyExpr, valueExpr := range hashPattern.Pairs {
				// Ключ должен быть строкой
				keyStr, ok := keyExpr.(*ast.StringLiteral)
				if !ok {
					return newErrorWithToken(node.Token, "ключи в паттерне деструктуризации должны быть строками")
				}
				
				// Значение должно быть идентификатором
				ident, ok := valueExpr.(*ast.Identifier)
				if !ok {
					return newErrorWithToken(node.Token, "значения в паттерне деструктуризации должны быть идентификаторами")
				}
				
				// Ищем значение в хеше
				hashKey := &String{Value: keyStr.Value}
				if pair, ok := hash.Pairs[hashKey.HashKey()]; ok {
					env.SetImmutable(ident.Value, pair.Value)
				} else {
					env.SetImmutable(ident.Value, NULL)
				}
			}
			return val
		}
		
		return newErrorWithToken(node.Token, "неверный паттерн деструктуризации")

	case *ast.VarStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val)
		return val

	case *ast.AssignmentStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		
		// Проверяем тип левой части
		switch left := node.Left.(type) {
		case *ast.Identifier:
			// Простое присваивание: имя = значение
			if env.IsImmutable(left.Value) {
				return newErrorWithToken(node.Token, "нельзя изменить immutable переменную '%s'", left.Value)
			}
			_, ok := env.Update(left.Value, val)
			if !ok {
				return newErrorWithToken(node.Token, "переменная '%s' не объявлена", left.Value)
			}
			return val
			
		case *ast.IndexExpression:
			// Присваивание через индекс: obj[key] = value или obj.field = value
			obj := Eval(left.Left, env)
			if isError(obj) {
				return obj
			}
			
			index := Eval(left.Index, env)
			if isError(index) {
				return index
			}
			
			// Устанавливаем значение
			switch o := obj.(type) {
			case *Hash:
				key, ok := index.(Hashable)
				if !ok {
					return newErrorWithToken(node.Token, "ключ хеша должен быть hashable")
				}
				o.Pairs[key.HashKey()] = HashPair{Key: index, Value: val}
				return val
				
			case *Instance:
				if index.Type() != "STRING" {
					return newErrorWithToken(node.Token, "индекс экземпляра должен быть STRING")
				}
				o.Properties[index.(*String).Value] = val
				return val
				
			case *Array:
				if index.Type() != "INTEGER" {
					return newErrorWithToken(node.Token, "индекс массива должен быть INTEGER")
				}
				idx := index.(*Integer).Value
				if idx < 0 || idx >= int64(len(o.Elements)) {
					return newErrorWithToken(node.Token, "индекс вне диапазона")
				}
				o.Elements[idx] = val
				return val
				
			default:
				return newErrorWithToken(node.Token, "присваивание не поддерживается для типа %s", obj.Type())
			}
			
		default:
			return newErrorWithToken(node.Token, "неверная левая часть присваивания")
		}

	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &ReturnValue{Value: val}

	case *ast.ImportStatement:
		return evalImportStatement(node, env)

	case *ast.ExportStatement:
		return evalExportStatement(node, env)

	case *ast.ThrowStatement:
		// Если нет значения (re-throw), ищем текущую ошибку в окружении
		if node.Value == nil {
			// Re-throw: ищем последнюю ошибку
			// Для простоты просто создаем новую ошибку
			return newError("повторный бросок ошибки")
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		// Преобразуем значение в ошибку
		if val.Type() == "ERROR_VALUE" {
			return &Error{Message: val.(*ErrorValue).Message}
		}
		return newError(val.Inspect())

	case *ast.BreakStatement:
		return &Break{}

	case *ast.ContinueStatement:
		return &Continue{}

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)

	// Expressions
	case *ast.IntegerLiteral:
		return &Integer{Value: node.Value}

	case *ast.FloatLiteral:
		return &Float{Value: node.Value}

	case *ast.StringLiteral:
		// Проверяем интерполяцию (маркер \x00 в начале)
		if len(node.Value) > 0 && node.Value[0] == '\x00' {
			return evalInterpolatedString(node.Value[1:], env)
		}
		return &String{Value: node.Value}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)

	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)

	case *ast.IfExpression:
		return evalIfExpression(node, env)

	case *ast.MatchExpression:
		return evalMatchExpression(node, env)

	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		fn := &Function{Parameters: params, Env: env, Body: body}
		if node.Name != nil {
			env.Set(node.Name.Value, fn)
		}
		return fn

	case *ast.CallExpression:
		// Специальная обработка для загрузить()
		if ident, ok := node.Function.(*ast.Identifier); ok && ident.Value == "загрузить" {
			if len(node.Arguments) != 1 {
				return newError("загрузить() требует 1 аргумент")
			}
			arg := Eval(node.Arguments[0], env)
			if isError(arg) {
				return arg
			}
			if arg.Type() != "STRING" {
				return newError("аргумент загрузить() должен быть STRING")
			}
			return evalLoad(arg.(*String).Value, env)
		}
		
		// Проверяем вызов метода (obj.method())
		if indexExpr, ok := node.Function.(*ast.IndexExpression); ok {
			left := Eval(indexExpr.Left, env)
			if isError(left) {
				return left
			}
			
			// Для Instance - вызываем метод из класса
			if left.Type() == "INSTANCE" {
				function := evalIndexExpression(left, Eval(indexExpr.Index, env))
				if isError(function) {
					return function
				}
				args := evalExpressions(node.Arguments, env)
				if len(args) == 1 && isError(args[0]) {
					return args[0]
				}
				allArgs := []Object{left}
				allArgs = append(allArgs, args...)
				return applyFunction(function, allArgs, node.Token)
			}
			
			// Для Hash - вызываем функцию из хеша
			if left.Type() == "HASH" {
				function := evalIndexExpression(left, Eval(indexExpr.Index, env))
				if isError(function) {
					return function
				}
				if function.Type() == "FUNCTION" {
					args := evalExpressions(node.Arguments, env)
					if len(args) == 1 && isError(args[0]) {
						return args[0]
					}
					return applyFunction(function, args, node.Token)
				}
			}
			
			// Для остальных типов - вызываем builtin функцию
			if indexObj := Eval(indexExpr.Index, env); indexObj.Type() == "STRING" {
				methodName := indexObj.(*String).Value
				
				// Сначала проверяем env
				if fn, ok := env.Get(methodName); ok {
					args := evalExpressions(node.Arguments, env)
					if len(args) == 1 && isError(args[0]) {
						return args[0]
					}
					allArgs := []Object{left}
					allArgs = append(allArgs, args...)
					return applyFunction(fn, allArgs, node.Token)
				}
				
				// Потом проверяем builtins
				if fn, ok := builtins[methodName]; ok {
					args := evalExpressions(node.Arguments, env)
					if len(args) == 1 && isError(args[0]) {
						return args[0]
					}
					allArgs := []Object{left}
					allArgs = append(allArgs, args...)
					return applyFunction(fn, allArgs, node.Token)
				}
			}
		}
		
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args, node.Function.GetToken())

	case *ast.ArrayLiteral:
		var result []Object
		for _, elem := range node.Elements {
			// Проверяем на spread
			if spreadExpr, ok := elem.(*ast.SpreadExpression); ok {
				spreadValue := Eval(spreadExpr.Value, env)
				if isError(spreadValue) {
					return spreadValue
				}
				// Разворачиваем массив
				if arr, ok := spreadValue.(*Array); ok {
					result = append(result, arr.Elements...)
				} else {
					return newError("spread можно применять только к массивам, получено %s", spreadValue.Type())
				}
			} else {
				val := Eval(elem, env)
				if isError(val) {
					return val
				}
				result = append(result, val)
			}
		}
		return &Array{Elements: result}

	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)

	case *ast.OptionalExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		
		// Если left это нет или NULL, возвращаем нет
		if left == NULL || left == FALSE {
			return FALSE
		}
		
		// Иначе вычисляем доступ к полю или вызов метода
		switch right := node.Right.(type) {
		case *ast.Identifier:
			// Доступ к полю: obj?.field
			index := &String{Value: right.Value}
			return evalIndexExpression(left, index)
			
		case *ast.CallExpression:
			// Вызов метода: obj?.method()
			if ident, ok := right.Function.(*ast.Identifier); ok {
				// Получаем метод из объекта
				methodName := &String{Value: ident.Value}
				method := evalIndexExpression(left, methodName)
				if isError(method) {
					return method
				}
				
				// Вызываем метод
				args := evalExpressions(right.Arguments, env)
				if len(args) == 1 && isError(args[0]) {
					return args[0]
				}
				return applyFunction(method, args, node.Token)
			}
		}
		
		return newErrorWithToken(node.Token, "неверное использование optional chaining")

	case *ast.ForExpression:
		return evalForExpression(node, env)

	case *ast.ForInExpression:
		return evalForInExpression(node, env)

	case *ast.WhileExpression:
		return evalWhileExpression(node, env)

	case *ast.HashLiteral:
		return evalHashLiteral(node, env)

	case *ast.TryExpression:
		return evalTryExpression(node, env)
	
	case *ast.NewExpression:
		return evalNewExpression(node, env)
	
	case *ast.TernaryExpression:
		return evalTernaryExpression(node, env)
	
	case *ast.RangeExpression:
		return evalRangeExpression(node, env)
	
	case *ast.ArrayComprehension:
		return evalArrayComprehension(node, env)
	}

	return NULL
}

func evalProgram(stmts []ast.Statement, env *Environment) Object {
	var result Object

	for _, statement := range stmts {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *ReturnValue:
			return result.Value
		case *Error:
			return result
		}
	}

	return result
}

func evalBlockStatement(block *ast.BlockStatement, env *Environment) Object {
	var result Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == "RETURN_VALUE" || rt == "ERROR" || rt == "BREAK" || rt == "CONTINUE" {
				return result
			}
		}
	}

	return result
}

func nativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right Object) Object {
	switch operator {
	case "не", "!":
		return evalNotOperator(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("неизвестный оператор: %s%s", operator, right.Type())
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

func evalMinusPrefixOperatorExpression(right Object) Object {
	if right.Type() == "INTEGER" {
		value := right.(*Integer).Value
		return &Integer{Value: -value}
	}
	if right.Type() == "FLOAT" {
		value := right.(*Float).Value
		return &Float{Value: -value}
	}
	return newError("неизвестный оператор: -%s", right.Type())
}

func evalInfixExpression(operator string, left, right Object) Object {
	switch {
	case left.Type() == "INTEGER" && right.Type() == "INTEGER":
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == "FLOAT" || right.Type() == "FLOAT":
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == "STRING" || right.Type() == "STRING" || left.Type() == "ERROR_VALUE" || right.Type() == "ERROR_VALUE":
		return evalStringInfixExpression(operator, left, right)
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case operator == "и":
		return nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))
	case operator == "или":
		return nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right))
	case left.Type() != right.Type():
		return newError("несовпадение типов: %s %s %s", left.Type(), operator, right.Type())
	default:
		return newError("неизвестный оператор: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(operator string, left, right Object) Object {
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
			return newError("деление на ноль")
		}
		return &Integer{Value: leftVal / rightVal}
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
		return newError("неизвестный оператор: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(operator string, left, right Object) Object {
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
			return newError("деление на ноль")
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
		return newError("неизвестный оператор: FLOAT %s FLOAT", operator)
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

	// Для оператора + поддерживаем конкатенацию с любыми типами
	leftVal := ""
	rightVal := ""
	
	// Преобразуем левую часть
	switch left.Type() {
	case "STRING":
		leftVal = left.(*String).Value
	case "INTEGER":
		leftVal = fmt.Sprintf("%d", left.(*Integer).Value)
	case "FLOAT":
		leftVal = fmt.Sprintf("%g", left.(*Float).Value)
	case "BOOLEAN":
		leftVal = left.Inspect()
	case "ARRAY":
		leftVal = left.Inspect()
	case "ERROR_VALUE":
		leftVal = left.(*ErrorValue).Message
	default:
		leftVal = left.Inspect()
	}
	
	// Преобразуем правую часть
	switch right.Type() {
	case "STRING":
		rightVal = right.(*String).Value
	case "INTEGER":
		rightVal = fmt.Sprintf("%d", right.(*Integer).Value)
	case "FLOAT":
		rightVal = fmt.Sprintf("%g", right.(*Float).Value)
	case "BOOLEAN":
		rightVal = right.Inspect()
	case "ARRAY":
		rightVal = right.Inspect()
	case "ERROR_VALUE":
		rightVal = right.(*ErrorValue).Message
	default:
		rightVal = right.Inspect()
	}

	return &String{Value: leftVal + rightVal}
}

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
		
		// Сравниваем значение с паттерном
		if compareObjects(value, pattern) {
			return Eval(matchCase.Result, env)
		}
	}
	
	// Если ничего не совпало, возвращаем нет
	return FALSE
}

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

	var result Object = NULL
	for i := fromVal; i <= toVal; i++ {
		env.Set(fe.Variable.Value, &Integer{Value: i})
		result = Eval(fe.Body, env)
		if isError(result) {
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
				env.Set(fie.Index.Value, &Integer{Value: int64(idx)})
			}
			env.Set(fie.Variable.Value, element)
			result = Eval(fie.Body, env)
			if isError(result) {
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
		idx := 0
		for _, pair := range iter.Pairs {
			if fie.Index != nil {
				env.Set(fie.Index.Value, &Integer{Value: int64(idx)})
				idx++
			}
			env.Set(fie.Variable.Value, pair.Value)
			result = Eval(fie.Body, env)
			if isError(result) {
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
				env.Set(fie.Index.Value, &Integer{Value: int64(idx)})
			}
			env.Set(fie.Variable.Value, &String{Value: string(ch)})
			result = Eval(fie.Body, env)
			if isError(result) {
				return result
			}
			if result.Type() == "BREAK" {
				return NULL
			}
			if result.Type() == "CONTINUE" {
				continue
			}
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
		if result.Type() == "BREAK" {
			return NULL
		}
		if result.Type() == "CONTINUE" {
			continue
		}
	}

	return result
}

func isTruthy(obj Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

func evalIdentifier(node *ast.Identifier, env *Environment) Object {
	// Проверяем переменные
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	// Проверяем встроенные функции
	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}

	return newErrorWithToken(node.Token, "идентификатор не найден: " + node.Value)
}

func evalExpressions(exps []ast.Expression, env *Environment) []Object {
	var result []Object

	for _, e := range exps {
		// Проверяем на spread
		if spreadExpr, ok := e.(*ast.SpreadExpression); ok {
			spreadValue := Eval(spreadExpr.Value, env)
			if isError(spreadValue) {
				return []Object{spreadValue}
			}
			// Разворачиваем массив
			if arr, ok := spreadValue.(*Array); ok {
				result = append(result, arr.Elements...)
			} else {
				return []Object{newError("spread можно применять только к массивам, получено %s", spreadValue.Type())}
			}
		} else {
			evaluated := Eval(e, env)
			if isError(evaluated) {
				return []Object{evaluated}
			}
			result = append(result, evaluated)
		}
	}

	return result
}

func applyFunction(fn Object, args []Object, tok lexer.Token) Object {
	switch fn := fn.(type) {
	case *Function:
		// Получаем текущую глубину из окружения функции
		currentDepth := fn.Env.callDepth
		
		// Проверка глубины рекурсии
		if currentDepth >= MaxCallDepth {
			return newErrorWithToken(tok, "превышена максимальная глубина рекурсии (%d). Возможно, функция вызывает сама себя бесконечно или имя функции совпадает с встроенной (добавить, удалить, вставить и т.д.)", MaxCallDepth)
		}
		
		// Создаем новое окружение с увеличенной глубиной
		extendedEnv := extendFunctionEnv(fn, args)
		extendedEnv.callDepth = currentDepth + 1
		
		// Временно обновляем окружение функции для рекурсивных вызовов
		oldEnv := fn.Env
		fn.Env = extendedEnv
		
		if len(args) > 0 && args[0].Type() == "INSTANCE" {
			extendedEnv.Set("это", args[0])
			args = args[1:]
		}
		
		for paramIdx, param := range fn.Parameters {
			if paramIdx < len(args) {
				extendedEnv.Set(param.Value, args[paramIdx])
			}
		}
		
		evaluated := Eval(fn.Body, extendedEnv)
		
		// Восстанавливаем окружение
		fn.Env = oldEnv
		
		return unwrapReturnValue(evaluated)
	case *Builtin:
		currentCallToken = tok
		return fn.Fn(args...)
	default:
		return newErrorWithToken(tok, "не функция: %s", fn.Type())
	}
}

func extendFunctionEnv(fn *Function, args []Object) *Environment {
	env := NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		if paramIdx < len(args) {
			env.Set(param.Value, args[paramIdx])
		}
	}

	return env
}

func unwrapReturnValue(obj Object) Object {
	if returnValue, ok := obj.(*ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func evalIndexExpression(left, index Object) Object {
	switch {
	case left.Type() == "ARRAY" && index.Type() == "INTEGER":
		return evalArrayIndexExpression(left, index)
	case left.Type() == "HASH":
		return evalHashIndexExpression(left, index)
	case left.Type() == "INSTANCE":
		return evalInstanceIndexExpression(left, index)
	case index.Type() == "STRING":
		// Это вызов метода - вернем функцию-обертку
		return evalMethodAccess(left, index)
	default:
		return newError("индексация не поддерживается: %s", left.Type())
	}
}

func evalMethodAccess(obj, methodName Object) Object {
	// Возвращаем builtin функцию, которая будет вызвана с obj как первым аргументом
	name := methodName.(*String).Value
	if builtin, ok := builtins[name]; ok {
		return builtin
	}
	return newError("метод не найден: %s", name)
}

func evalInstanceIndexExpression(instance, index Object) Object {
	inst := instance.(*Instance)
	
	if index.Type() != "STRING" {
		return newError("индекс экземпляра должен быть STRING")
	}
	
	key := index.(*String).Value
	
	if val, ok := inst.Properties[key]; ok {
		return val
	}
	
	keyObj := &String{Value: key}
	if pair, ok := inst.Class.Pairs[keyObj.HashKey()]; ok {
		return pair.Value
	}
	
	return NULL
}

func evalArrayIndexExpression(array, index Object) Object {
	arrayObject := array.(*Array)
	idx := index.(*Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

func evalHashIndexExpression(hash, index Object) Object {
	hashObject := hash.(*Hash)

	key, ok := index.(Hashable)
	if !ok {
		return newError("непригодный ключ для хеша: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}

	return pair.Value
}

func evalHashLiteral(node *ast.HashLiteral, env *Environment) Object {
	pairs := make(map[HashKey]HashPair)

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}

		hashKey, ok := key.(Hashable)
		if !ok {
			return newError("непригодный ключ для хеша: %s", key.Type())
		}

		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}

		hashed := hashKey.HashKey()
		pairs[hashed] = HashPair{Key: key, Value: value}
	}

	return &Hash{Pairs: pairs}
}

func evalTryExpression(te *ast.TryExpression, env *Environment) Object {
	// Выполняем тело попытки
	result := evalBlockStatement(te.Body, env)
	
	// Если встретили ошибку и есть catch блок
	if isError(result) && te.CatchBody != nil {
		catchEnv := NewEnclosedEnvironment(env)
		
		// Если указана переменная для ошибки
		if te.CatchVar != nil {
			// Преобразуем Error в ErrorValue чтобы не пропагировалась как ошибка
			errorValue := &ErrorValue{Message: result.(*Error).Message}
			catchEnv.Set(te.CatchVar.Value, errorValue)
		}
		
		// Выполняем catch блок
		result = evalBlockStatement(te.CatchBody, catchEnv)
	}

	// Выполняем finally блок (всегда)
	if te.FinallyBody != nil {
		finallyResult := evalBlockStatement(te.FinallyBody, env)
		// Если в finally произошла ошибка или return, она имеет приоритет
		if finallyResult != nil && (finallyResult.Type() == "ERROR" || finallyResult.Type() == "RETURN_VALUE") {
			return finallyResult
		}
	}

	return result
}

func evalInterpolatedString(template string, env *Environment) Object {
	var result strings.Builder
	
	for i := 0; i < len(template); i++ {
		if template[i] == '{' {
			// Находим закрывающую скобку
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
				// Извлекаем выражение
				exprStr := template[i+1 : j-1]
				
				if len(exprStr) == 0 {
					return newError("пустое выражение в интерполяции")
				}
				
				// Парсим и вычисляем выражение
				l := lexer.New(exprStr)
				p := parser.New(l)
				program := p.ParseProgram()
				
				if len(p.Errors()) > 0 {
					return newError("ошибка парсинга в интерполяции '{%s}': %s", exprStr, p.Errors()[0])
				}
				
				if len(program.Statements) == 0 {
					return newError("пустое выражение в интерполяции")
				}
				
				// Вычисляем первое выражение
				stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
				if !ok {
					return newError("ожидалось выражение в интерполяции, получено: %T", program.Statements[0])
				}
				
				val := Eval(stmt.Expression, env)
				if isError(val) {
					return val
				}
				
				// Добавляем в результат
				result.WriteString(val.Inspect())
				i = j - 1
				continue
			}
		}
		
		result.WriteByte(template[i])
	}
	
	return &String{Value: result.String()}
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func newErrorWithToken(tok lexer.Token, format string, a ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf(format, a...),
		Line:    tok.Line,
		Column:  tok.Column,
	}
}

func isError(obj Object) bool {
	if obj != nil {
		return obj.Type() == "ERROR"
	}
	return false
}

func evalLoad(path string, env *Environment) Object {
	// Читаем файл
	content, err := os.ReadFile(path)
	if err != nil {
		return newError("ошибка загрузки файла: %s", err.Error())
	}

	// Парсим
	l := lexer.New(string(content))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		errMsg := "ошибки парсинга в " + path + ":"
		for _, msg := range p.Errors() {
			errMsg += "\n  " + msg
		}
		return newError(errMsg)
	}

	// Выполняем в текущем окружении
	return Eval(program, env)
}

func evalNewExpression(node *ast.NewExpression, env *Environment) Object {
	classObj, ok := env.Get(node.ClassName.Value)
	if !ok {
		return newError("класс не найден: %s", node.ClassName.Value)
	}
	
	if classObj.Type() != "HASH" {
		return newError("%s не является классом", node.ClassName.Value)
	}
	
	class := classObj.(*Hash)
	
	instance := &Instance{
		Class:      class,
		Properties: make(map[string]Object),
	}
	
	// Ищем конструктор: "инициализация" или "создать"
	initKey := &String{Value: "инициализация"}
	createKey := &String{Value: "создать"}
	
	var constructorPair *HashPair
	if pair, ok := class.Pairs[initKey.HashKey()]; ok {
		constructorPair = &pair
	} else if pair, ok := class.Pairs[createKey.HashKey()]; ok {
		constructorPair = &pair
	}
	
	if constructorPair != nil {
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		
		if fn, ok := constructorPair.Value.(*Function); ok {
			extendedEnv := NewEnclosedEnvironment(fn.Env)
			extendedEnv.Set("это", instance)
			
			for i, param := range fn.Parameters {
				if i < len(args) {
					extendedEnv.Set(param.Value, args[i])
				}
			}
			
			Eval(fn.Body, extendedEnv)
		}
	}
	
	return instance
}

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

func applyFunctionFromBuiltin(fn Object, args []Object) Object {
	switch fn := fn.(type) {
	case *Function:
		extendedEnv := NewEnclosedEnvironment(fn.Env)
		
		for paramIdx, param := range fn.Parameters {
			if paramIdx < len(args) {
				extendedEnv.Set(param.Value, args[paramIdx])
			}
		}
		
		evaluated := Eval(fn.Body, extendedEnv)
		if returnValue, ok := evaluated.(*ReturnValue); ok {
			return returnValue.Value
		}
		return evaluated
	default:
		return newError("не функция: %s", fn.Type())
	}
}


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
		elements = append(elements, &Integer{Value: i})
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
			
			// Проверяем условие если есть
			if node.Condition != nil {
				condition := Eval(node.Condition, env)
				if isError(condition) {
					return condition
				}
				if !isTruthy(condition) {
					continue
				}
			}
			
			// Вычисляем выражение
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

func evalImportStatement(node *ast.ImportStatement, env *Environment) Object {
	// Загружаем модуль из файла
	path := node.Path
	
	// Читаем файл
	content, err := os.ReadFile(path)
	if err != nil {
		return newErrorWithToken(node.Token, "не удалось прочитать файл: %s", err.Error())
	}
	
	// Парсим и выполняем
	l := lexer.New(string(content))
	p := parser.New(l)
	program := p.ParseProgram()
	
	if len(p.Errors()) > 0 {
		return newErrorWithToken(node.Token, "ошибка парсинга модуля: %s", p.Errors()[0])
	}
	
	// Создаем новое окружение для модуля
	moduleEnv := NewEnvironment()
	moduleEnv.outer = env // Доступ к глобальным переменным
	
	// Выполняем модуль
	result := Eval(program, moduleEnv)
	if isError(result) {
		return result
	}
	
	// Создаем объект модуля с экспортированными значениями
	moduleObj := &Hash{Pairs: make(map[HashKey]HashPair)}
	
	// Собираем все экспортированные значения
	for k, v := range moduleEnv.exports {
		key := &String{Value: k}
		moduleObj.Pairs[key.HashKey()] = HashPair{Key: key, Value: v}
	}
	
	// Сохраняем модуль в окружении
	moduleName := node.Name.Value
	if node.Alias != nil {
		moduleName = node.Alias.Value
	}
	env.Set(moduleName, moduleObj)
	
	return moduleObj
}

func evalExportStatement(node *ast.ExportStatement, env *Environment) Object {
	// Выполняем statement
	result := Eval(node.Statement, env)
	if isError(result) {
		return result
	}
	
	// Добавляем в exports
	switch stmt := node.Statement.(type) {
	case *ast.LetStatement:
		env.Export(stmt.Name.Value)
	case *ast.ExpressionStatement:
		// Может быть функция
		if fn, ok := stmt.Expression.(*ast.FunctionLiteral); ok {
			if fn.Name != nil {
				env.Export(fn.Name.Value)
			}
		}
	}
	
	return result
}

