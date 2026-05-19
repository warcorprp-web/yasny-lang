package interpreter

import (
	"fmt"
	"yasny-lang/ast"
	"yasny-lang/lexer"
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
		if err := bindPattern(node.Pattern, val, env, node.Token); err != nil {
			return err
		}
		return val

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
				if _, ok := index.(Hashable); !ok {
					return newErrorWithToken(node.Token, "ключ хеша должен быть hashable")
				}
				o.Set(index, val)
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
		return newError("%s", val.Inspect())

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

	case *ast.NullLiteral:
		return NULL

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Token, node.Operator, right)

	case *ast.InfixExpression:
		// Специальная обработка для ?? (nullish coalescing, ленивое вычисление)
		if node.Operator == "??" {
			left := Eval(node.Left, env)
			if isError(left) {
				return left
			}
			// Если left не пусто/нет - возвращаем его, иначе вычисляем right
			if left == NULL || left == FALSE {
				return Eval(node.Right, env)
			}
			return left
		}
		
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
		fn := &Function{
			Parameters:  params,
			Defaults:    node.Defaults,
			HasRest:     node.HasRest,
			Env:         env,
			Body:        body,
			IsGenerator: containsYield(body),
		}
		if node.Name != nil {
			env.Set(node.Name.Value, fn)
		}
		return fn

	case *ast.YieldStatement:
		// Внутри генератора эмитим значение в канал
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		// Канал генератора хранится в env по специальному ключу
		if chObj, ok := env.Get("__gen_ch__"); ok {
			if ch, ok := chObj.(*GenChannel); ok {
				select {
				case ch.Ch <- val:
					// успешно отправили
				case <-ch.Done:
					// генератор закрыт - тихо завершаем
					return &generatorStopSignal{}
				}
			}
		}
		return NULL
	
	case *ast.AsyncExpression:
		// Запускаем тело в горутине, возвращаем Future
		future := &Future{Done: make(chan struct{})}
		// Капчуем env для горутины
		goEnv := env
		go func() {
			defer func() {
				if r := recover(); r != nil {
					future.Result = newError("асинхронная ошибка: %v", r)
				}
				close(future.Done)
			}()
			result := Eval(node.Body, goEnv)
			future.Result = result
		}()
		return future
	
	case *ast.AwaitExpression:
		val := Eval(node.Body, env)
		if isError(val) {
			return val
		}
		if fut, ok := val.(*Future); ok {
			return fut.Wait()
		}
		// Ждать не-future - просто возвращаем как есть
		return val

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
