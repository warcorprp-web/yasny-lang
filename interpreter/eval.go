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
				return ErrorInvalidDestructuring(node.Token, "массива", val.Type())
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
						return ErrorWithHint(
							node.Token,
							"rest оператор (...) должен содержать имя переменной",
							"Используйте: конст [первый, ...остальные] = массив",
						)
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
					return ErrorWithHint(
						node.Token,
						"паттерн деструктуризации должен содержать только имена переменных",
						"Используйте: конст [а, б, в] = массив",
					)
				}
			}
			return val
		}
		
		// Деструктуризация объекта: конст {x, y} = объект
		if hashPattern, ok := node.Pattern.(*ast.HashLiteral); ok {
			if val.Type() != "HASH" {
				return ErrorInvalidDestructuring(node.Token, "объекта", val.Type())
			}
			
			hash := val.(*Hash)
			for keyExpr, valueExpr := range hashPattern.Pairs {
				// Ключ должен быть строкой
				keyStr, ok := keyExpr.(*ast.StringLiteral)
				if !ok {
					return ErrorWithHint(
						node.Token,
						"ключи в паттерне деструктуризации должны быть строками",
						"Используйте: конст {\"ключ\": переменная} = объект",
					)
				}
				
				// Значение должно быть идентификатором
				ident, ok := valueExpr.(*ast.Identifier)
				if !ok {
					return ErrorWithHint(
						node.Token,
						"значения в паттерне деструктуризации должны быть именами переменных",
						"Используйте: конст {\"ключ\": переменная} = объект",
					)
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
		
		return ErrorWithHint(
			node.Token,
			"неверный паттерн деструктуризации",
			"Используйте: конст [а, б] = массив или конст {\"ключ\": значение} = объект",
		)

	case *ast.VarStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val)
		return val

	case *ast.AssignmentStatement:
		// Для операторов +=, -=, *=, /= нужно сначала получить текущее значение
		var val Object
		
		if node.Operator != "=" {
			// Получаем текущее значение
			var currentVal Object
			switch left := node.Left.(type) {
			case *ast.Identifier:
				var ok bool
				currentVal, ok = env.Get(left.Value)
				if !ok {
					return ErrorVariableNotDeclared(node.Token, left.Value)
				}
			case *ast.IndexExpression:
				obj := Eval(left.Left, env)
				if isError(obj) {
					return obj
				}
				index := Eval(left.Index, env)
				if isError(index) {
					return index
				}
				currentVal = evalIndexExpression(left.Token, obj, index)
				if isError(currentVal) {
					return currentVal
				}
			}
			
			// Вычисляем новое значение
			rightVal := Eval(node.Value, env)
			if isError(rightVal) {
				return rightVal
			}
			
			// Применяем оператор
			var operator string
			switch node.Operator {
			case "+=":
				operator = "+"
			case "-=":
				operator = "-"
			case "*=":
				operator = "*"
			case "/=":
				operator = "/"
			}
			
			val = evalInfixExpression(node.Token, operator, currentVal, rightVal)
			if isError(val) {
				return val
			}
		} else {
			// Обычное присваивание
			val = Eval(node.Value, env)
			if isError(val) {
				return val
			}
		}
		
		// Проверяем тип левой части
		switch left := node.Left.(type) {
		case *ast.Identifier:
			// Простое присваивание: имя = значение
			if env.IsImmutable(left.Value) {
				return ErrorCannotReassignConst(node.Token, left.Value)
			}
			_, ok := env.Update(left.Value, val)
			if !ok {
				return ErrorVariableNotDeclared(node.Token, left.Value)
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
					return ErrorIndexOutOfRange(node.Token, idx, len(o.Elements))
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
		return evalPrefixExpression(node.Token, node.Operator, right)

	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Token, node.Operator, left, right)

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
				return ErrorWithHint(
					node.Token,
					"функция 'загрузить' ожидает 1 аргумент (путь к файлу)",
					"Используйте: загрузить(\"путь/к/файлу.ya\")",
				)
			}
			arg := Eval(node.Arguments[0], env)
			if isError(arg) {
				return arg
			}
			if arg.Type() != "STRING" {
				return ErrorWithHint(
					node.Token,
					"аргумент функции 'загрузить' должен быть строкой",
					"Используйте: загрузить(\"путь/к/файлу.ya\")",
				)
			}
			return evalLoad(node.Token, arg.(*String).Value, env)
		}
		
		// Проверяем вызов метода (obj.method())
		if indexExpr, ok := node.Function.(*ast.IndexExpression); ok {
			left := Eval(indexExpr.Left, env)
			if isError(left) {
				return left
			}
			
			// Для Instance - вызываем метод из класса
			if left.Type() == "INSTANCE" {
				function := evalIndexExpression(indexExpr.Token, left, Eval(indexExpr.Index, env))
				if isError(function) {
					return function
				}
				args := evalExpressions(node.Arguments, env)
				if len(args) == 1 && isError(args[0]) {
					return args[0]
				}
				allArgs := []Object{left}
				allArgs = append(allArgs, args...)
				return applyFunction(function, allArgs, node.Token, env)
			}
			
			// Для Hash - вызываем функцию из хеша (например родитель.инициализация или время.сейчас)
			if left.Type() == "HASH" {
				function := evalIndexExpression(indexExpr.Token, left, Eval(indexExpr.Index, env))
				if isError(function) {
					return function
				}
				if function.Type() == "FUNCTION" {
					args := evalExpressions(node.Arguments, env)
					if len(args) == 1 && isError(args[0]) {
						return args[0]
					}
					
					// Если вызывается из контекста класса, передаем это
					fn := function.(*Function)
					extendedEnv := NewEnvironment()
					extendedEnv.outer = fn.Env
					
					// Проверяем есть ли это в текущем окружении
					if thisObj, ok := env.Get("это"); ok {
						extendedEnv.Set("это", thisObj)
					}
					
					// Добавляем параметры
					for paramIdx, param := range fn.Parameters {
						if paramIdx < len(args) {
							extendedEnv.Set(param.Value, args[paramIdx])
						}
					}
					
					result := Eval(fn.Body, extendedEnv)
					if returnValue, ok := result.(*ReturnValue); ok {
						return returnValue.Value
					}
					return result
				}
				// Builtin (для модулей время, мат и т.д.)
				if function.Type() == "BUILTIN" {
					args := evalExpressions(node.Arguments, env)
					if len(args) == 1 && isError(args[0]) {
						return args[0]
					}
					return applyFunction(function, args, node.Token, env)
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
					return applyFunction(fn, allArgs, node.Token, env)
				}
				
				// Потом проверяем builtins
				if fn, ok := builtins[methodName]; ok {
					args := evalExpressions(node.Arguments, env)
					if len(args) == 1 && isError(args[0]) {
						return args[0]
					}
					allArgs := []Object{left}
					allArgs = append(allArgs, args...)
					return applyFunction(fn, allArgs, node.Token, env)
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
		return applyFunction(function, args, node.Function.GetToken(), env)

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
					return ErrorSpreadNotArray(spreadExpr.Token, spreadValue.Type())
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
		return evalIndexExpression(node.Token, left, index)

	case *ast.SliceExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		var startIdx, endIdx Object
		if node.Start != nil {
			startIdx = Eval(node.Start, env)
			if isError(startIdx) {
				return startIdx
			}
		}
		if node.End != nil {
			endIdx = Eval(node.End, env)
			if isError(endIdx) {
				return endIdx
			}
		}
		return evalSliceExpression(node.Token, left, startIdx, endIdx)

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
			return evalIndexExpression(node.Token, left, index)
			
		case *ast.CallExpression:
			// Вызов метода: obj?.method()
			if ident, ok := right.Function.(*ast.Identifier); ok {
				// Получаем метод из объекта
				methodName := &String{Value: ident.Value}
				method := evalIndexExpression(node.Token, left, methodName)
				if isError(method) {
					return method
				}
				
				// Вызываем метод
				args := evalExpressions(right.Arguments, env)
				if len(args) == 1 && isError(args[0]) {
					return args[0]
				}
				return applyFunction(method, args, node.Token, env)
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

func evalInfixExpression(tok lexer.Token, operator string, left, right Object) Object {
	switch {
	case left.Type() == "INTEGER" && right.Type() == "INTEGER":
		return evalIntegerInfixExpression(tok, operator, left, right)
	case left.Type() == "FLOAT" || right.Type() == "FLOAT":
		return evalFloatInfixExpression(tok, operator, left, right)
	case left.Type() == "STRING" && right.Type() == "STRING":
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == "ERROR_VALUE" || right.Type() == "ERROR_VALUE":
		return evalStringInfixExpression(operator, left, right)
	case operator == "==":
		// NULL == нет должно быть да
		if (left == NULL && right == FALSE) || (left == FALSE && right == NULL) {
			return TRUE
		}
		// Разные типы всегда не равны
		if left.Type() != right.Type() {
			return FALSE
		}
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		// NULL != нет должно быть нет
		if (left == NULL && right == FALSE) || (left == FALSE && right == NULL) {
			return FALSE
		}
		// Разные типы всегда не равны
		if left.Type() != right.Type() {
			return TRUE
		}
		return nativeBoolToBooleanObject(left != right)
	case operator == "и":
		return nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))
	case operator == "или":
		return nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right))
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
		// Если деление точное - возвращаем целое, иначе - дробное
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
		
		// Если паттерн - булево, а значение не булево, то это условие (guard)
		// Например: совпадение оценка ... когда оценка >= 90: "Отлично"
		if pattern.Type() == "BOOLEAN" && value.Type() != "BOOLEAN" {
			if pattern.(*Boolean).Value {
				return Eval(matchCase.Result, env)
			}
			continue
		}
		
		// Иначе - сравниваем значение с паттерном
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

	// Проверяем стандартные модули (время, мат, ...)
	if mod, ok := stdModules[node.Value]; ok {
		return mod
	}

	return ErrorIdentifierNotFound(node.Token, node.Value)
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
				return []Object{ErrorSpreadNotArray(spreadExpr.Token, spreadValue.Type())}
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

func applyFunction(fn Object, args []Object, tok lexer.Token, env *Environment) Object {
	switch fn := fn.(type) {
	case *Hash:
		// Проверяем, является ли это классом (есть метод инициализация или наследует)
		initKey := (&String{Value: "инициализация"}).HashKey()
		parentNameKey := (&String{Value: "__parent_name__"}).HashKey()
		
		// Получаем имя родительского класса если есть
		var parentName string
		var parentHash *Hash
		if parentNamePair, ok := fn.Pairs[parentNameKey]; ok {
			if parentNamePair.Value.Type() == "STRING" {
				parentName = parentNamePair.Value.(*String).Value
			}
		}
		if parentName != "" {
			if parentObj, found := env.Get(parentName); found && parentObj.Type() == "HASH" {
				parentHash = parentObj.(*Hash)
			}
		}
		
		// Ищем инициализация: сначала в текущем классе, потом по цепочке родителей
		initPair, hasInit := fn.Pairs[initKey]
		var inheritedInit bool
		if !hasInit {
			// Идём по цепочке наследования
			current := parentHash
			for current != nil {
				if pair, ok := current.Pairs[initKey]; ok {
					initPair = pair
					hasInit = true
					inheritedInit = true
					break
				}
				// Ищем родителя у current
				if pn, ok := current.Pairs[parentNameKey]; ok && pn.Value.Type() == "STRING" {
					if po, found := env.Get(pn.Value.(*String).Value); found && po.Type() == "HASH" {
						current = po.(*Hash)
						continue
					}
				}
				break
			}
		}
		
		// Это класс если есть инициализация (своя или унаследованная) или есть родитель
		isClass := hasInit || parentHash != nil
		if !isClass {
			return ErrorNotCallable(tok, fn.Type())
		}
		
		// Создаём экземпляр класса
		instance := &Instance{
			Class:      fn,
			Parent:     parentHash,
			ParentName: parentName,
			Properties: make(map[string]Object),
		}
		
		// Вызываем инициализацию (если есть)
		if hasInit {
			initMethod := initPair.Value
			if initMethod.Type() == "FUNCTION" {
				initFunc := initMethod.(*Function)
				extendedEnv := NewEnvironment()
				extendedEnv.outer = initFunc.Env
				extendedEnv.Set("это", instance)
				
				// Если унаследовали init - устанавливаем родителя родителя для super calls
				if inheritedInit {
					// в контексте родительского init "родитель" = дед
					if parentHash != nil {
						if pn, ok := parentHash.Pairs[parentNameKey]; ok && pn.Value.Type() == "STRING" {
							if po, found := env.Get(pn.Value.(*String).Value); found && po.Type() == "HASH" {
								extendedEnv.Set("родитель", po)
							}
						}
					}
				} else if parentName != "" {
					// Свой init - "родитель" указывает на родительский класс
					if parentObj, found := env.Get(parentName); found && parentObj.Type() == "HASH" {
						extendedEnv.Set("родитель", parentObj)
					}
				}
				
				for paramIdx, param := range initFunc.Parameters {
					if paramIdx < len(args) {
						extendedEnv.Set(param.Value, args[paramIdx])
					}
				}
				
				result := Eval(initFunc.Body, extendedEnv)
				if isError(result) {
					return result
				}
			}
		}
		
		return instance
	case *Function:
		// Получаем текущую глубину из окружения функции
		currentDepth := fn.Env.callDepth
		
		// Проверка глубины рекурсии
		if currentDepth >= MaxCallDepth {
			return ErrorWithHint(
				tok,
				fmt.Sprintf("превышена максимальная глубина рекурсии (%d)", MaxCallDepth),
				"Возможно, функция вызывает сама себя бесконечно. Проверьте условие выхода из рекурсии или убедитесь, что имя функции не совпадает с встроенной (добавить, удалить, вставить и т.д.).",
			)
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
		return ErrorNotCallable(tok, fn.Type())
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

func evalSliceExpression(tok lexer.Token, left, start, end Object) Object {
	// normalizeSlice возвращает реальные индексы start/end с учётом длины и nil
	normalize := func(length int64) (int64, int64, *Error) {
		var s, e int64
		s = 0
		e = length
		if start != nil {
			si, ok := start.(*Integer)
			if !ok {
				return 0, 0, ErrorWithHint(tok, "начало среза должно быть целым числом", "")
			}
			s = si.Value
			if s < 0 {
				s = length + s
			}
		}
		if end != nil {
			ei, ok := end.(*Integer)
			if !ok {
				return 0, 0, ErrorWithHint(tok, "конец среза должен быть целым числом", "")
			}
			e = ei.Value
			if e < 0 {
				e = length + e
			}
		}
		// Зажимаем границы
		if s < 0 {
			s = 0
		}
		if e > length {
			e = length
		}
		if s > e {
			s = e
		}
		return s, e, nil
	}

	switch left := left.(type) {
	case *Array:
		s, e, err := normalize(int64(len(left.Elements)))
		if err != nil {
			return err
		}
		return &Array{Elements: append([]Object{}, left.Elements[s:e]...)}
	case *String:
		runes := []rune(left.Value)
		s, e, err := normalize(int64(len(runes)))
		if err != nil {
			return err
		}
		return &String{Value: string(runes[s:e])}
	default:
		return ErrorWithHint(tok,
			fmt.Sprintf("срез не поддерживается для типа %s", translateType(left.Type())),
			"Срезы работают для массивов и строк.",
		)
	}
}

func evalIndexExpression(tok lexer.Token, left, index Object) Object {
	switch {
	case left.Type() == "ARRAY" && index.Type() == "INTEGER":
		return evalArrayIndexExpression(tok, left, index)
	case left.Type() == "HASH":
		return evalHashIndexExpression(tok, left, index)
	case left.Type() == "INSTANCE":
		return evalInstanceIndexExpression(tok, left, index)
	case index.Type() == "STRING":
		// Это вызов метода - вернем функцию-обертку
		return evalMethodAccess(tok, left, index)
	default:
		return ErrorWithHint(
			tok,
			fmt.Sprintf("индексация не поддерживается для типа %s", left.Type()),
			"Индексация работает только для массивов, объектов и экземпляров классов.",
		)
	}
}

func evalMethodAccess(tok lexer.Token, obj, methodName Object) Object {
	// Возвращаем builtin функцию, которая будет вызвана с obj как первым аргументом
	name := methodName.(*String).Value
	if builtin, ok := builtins[name]; ok {
		return builtin
	}
	
	// Определяем тип объекта для лучшего сообщения
	objType := "объект"
	if obj.Type() == "INSTANCE" {
		inst := obj.(*Instance)
		if className, ok := inst.Properties["__class__"]; ok {
			objType = fmt.Sprintf("класс '%s'", className.Inspect())
		}
	}
	
	return ErrorMethodNotFound(tok, name, objType)
}

func evalInstanceIndexExpression(tok lexer.Token, instance, index Object) Object {
	inst := instance.(*Instance)
	
	if index.Type() != "STRING" {
		return ErrorWithHint(
			tok,
			"индекс экземпляра должен быть строкой",
			"Используйте строку для доступа к свойствам: объект.свойство или объект[\"свойство\"]",
		)
	}
	
	key := index.(*String).Value
	
	// Сначала ищем в свойствах экземпляра
	if val, ok := inst.Properties[key]; ok {
		return val
	}
	
	// Потом в методах класса
	keyObj := &String{Value: key}
	if pair, ok := inst.Class.Pairs[keyObj.HashKey()]; ok {
		return pair.Value
	}
	
	// Потом в родительском классе
	if inst.Parent != nil {
		if pair, ok := inst.Parent.Pairs[keyObj.HashKey()]; ok {
			return pair.Value
		}
	}
	
	return NULL
}

func evalArrayIndexExpression(tok lexer.Token, array, index Object) Object {
	arrayObject := array.(*Array)
	idx := index.(*Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	// Поддержка отрицательных индексов (как в Python)
	if idx < 0 {
		idx = int64(len(arrayObject.Elements)) + idx
	}

	if idx < 0 || idx > max {
		return ErrorIndexOutOfRange(tok, index.(*Integer).Value, len(arrayObject.Elements))
	}

	return arrayObject.Elements[idx]
}

func evalHashIndexExpression(tok lexer.Token, hash, index Object) Object {
	hashObject := hash.(*Hash)

	key, ok := index.(Hashable)
	if !ok {
		return ErrorWithHint(
			tok,
			fmt.Sprintf("ключ хеша должен быть hashable типом, получен %s", index.Type()),
			"Используйте строки, числа или булевы значения в качестве ключей.",
		)
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
	location := ""
	if tok.Filename != "" {
		location = fmt.Sprintf("[%s:%d] ", tok.Filename, tok.Line)
	} else if tok.Line > 0 {
		location = fmt.Sprintf("[строка %d] ", tok.Line)
	}
	
	return &Error{
		Message: fmt.Sprintf("❌ ОШИБКА %s"+format, append([]interface{}{location}, a...)...),
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

func evalLoad(tok lexer.Token, path string, env *Environment) Object {
	// Читаем файл
	content, err := os.ReadFile(path)
	if err != nil {
		return ErrorFileNotFound(tok, path)
	}

	// Парсим
	l := lexer.NewWithFilename(string(content), path)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		// Показываем ошибки из загружаемого файла
		errMsg := fmt.Sprintf("при загрузке '%s':", path)
		for _, msg := range p.Errors() {
			errMsg += "\n  " + msg
		}
		return &Error{
			Message: fmt.Sprintf("❌ ОШИБКА %s", errMsg),
			Line:    tok.Line,
			Column:  tok.Column,
		}
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
	l := lexer.NewWithFilename(string(content), path)
	p := parser.New(l)
	program := p.ParseProgram()
	
	if len(p.Errors()) > 0 {
		// Показываем ошибку из импортируемого файла
		return &Error{
			Message: fmt.Sprintf("❌ ОШИБКА при импорте из '%s':\n  %s", path, p.Errors()[0]),
			Line:    node.Token.Line,
			Column:  node.Token.Column,
		}
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

