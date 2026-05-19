package interpreter

import (
	"fmt"

	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// evalIdentifier ищет значение по имени: сначала среди переменных,
// затем среди встроенных функций, затем среди стандартных модулей.
func evalIdentifier(node *ast.Identifier, env *Environment) Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}

	if mod, ok := stdModules[node.Value]; ok {
		return mod
	}

	return ErrorIdentifierNotFound(node.Token, node.Value)
}

// evalExpressions вычисляет список выражений, разворачивая spread (...).
func evalExpressions(exps []ast.Expression, env *Environment) []Object {
	var result []Object

	for _, e := range exps {
		if spreadExpr, ok := e.(*ast.SpreadExpression); ok {
			spreadValue := Eval(spreadExpr.Value, env)
			if isError(spreadValue) {
				return []Object{spreadValue}
			}
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

// applyFunction применяет вызов к объекту: классу (создаёт экземпляр),
// функции (выполняет тело) или builtin'у.
func applyFunction(fn Object, args []Object, tok lexer.Token, env *Environment) Object {
	switch fn := fn.(type) {
	case *Hash:
		// Класс распознаётся по наличию инициализации (своей или
		// унаследованной) или родителя.
		initKey := (&String{Value: "инициализация"}).HashKey()
		parentNameKey := (&String{Value: "__parent_name__"}).HashKey()

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

		// Ищем инициализатор по цепочке наследования.
		initPair, hasInit := fn.Pairs[initKey]
		var inheritedInit bool
		if !hasInit {
			current := parentHash
			for current != nil {
				if pair, ok := current.Pairs[initKey]; ok {
					initPair = pair
					hasInit = true
					inheritedInit = true
					break
				}
				if pn, ok := current.Pairs[parentNameKey]; ok && pn.Value.Type() == "STRING" {
					if po, found := env.Get(pn.Value.(*String).Value); found && po.Type() == "HASH" {
						current = po.(*Hash)
						continue
					}
				}
				break
			}
		}

		isClass := hasInit || parentHash != nil
		if !isClass {
			return ErrorNotCallable(tok, fn.Type())
		}

		instance := &Instance{
			Class:      fn,
			Parent:     parentHash,
			ParentName: parentName,
			Properties: make(map[string]Object),
		}

		if hasInit {
			initMethod := initPair.Value
			if initMethod.Type() == "FUNCTION" {
				initFunc := initMethod.(*Function)
				extendedEnv := NewEnvironment()
				extendedEnv.outer = initFunc.Env
				extendedEnv.Set("это", instance)

				// При наследовании 'родитель' внутри
				// унаследованного init указывает на дедушку,
				// иначе — на родителя.
				if inheritedInit {
					if parentHash != nil {
						if pn, ok := parentHash.Pairs[parentNameKey]; ok && pn.Value.Type() == "STRING" {
							if po, found := env.Get(pn.Value.(*String).Value); found && po.Type() == "HASH" {
								extendedEnv.Set("родитель", po)
							}
						}
					}
				} else if parentName != "" {
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
		// Генератор запускается в горутине, возвращает Generator.
		if fn.IsGenerator {
			ch := make(chan Object, 1)
			done := make(chan bool, 1)
			gen := &Generator{Ch: ch, Done: done}

			extendedEnv := extendFunctionEnv(fn, args)
			if len(args) > 0 && args[0].Type() == "INSTANCE" {
				extendedEnv.Set("это", args[0])
				args = args[1:]
			}
			for paramIdx, param := range fn.Parameters {
				if paramIdx < len(args) {
					extendedEnv.Set(param.Value, args[paramIdx])
				}
			}
			extendedEnv.Set("__gen_ch__", &GenChannel{Ch: ch, Done: done})

			go func() {
				defer func() {
					recover() // ловим panic от закрытого канала
					close(ch)
				}()
				Eval(fn.Body, extendedEnv)
			}()

			return gen
		}

		// Защита от бесконечной рекурсии.
		currentDepth := fn.Env.callDepth
		if currentDepth >= MaxCallDepth {
			return ErrorWithHint(
				tok,
				fmt.Sprintf("превышена максимальная глубина рекурсии (%d)", MaxCallDepth),
				"Возможно, функция вызывает сама себя бесконечно. Проверьте условие выхода из рекурсии или убедитесь, что имя функции не совпадает с встроенной (добавить, удалить, вставить и т.д.).",
			)
		}

		extendedEnv := NewEnclosedEnvironment(fn.Env)
		extendedEnv.callDepth = currentDepth + 1

		// Временно подменяем Env у функции, чтобы рекурсивные
		// вызовы видели увеличенную глубину.
		oldEnv := fn.Env
		fn.Env = extendedEnv

		if len(args) > 0 && args[0].Type() == "INSTANCE" {
			extendedEnv.Set("это", args[0])
			args = args[1:]
		}

		bindParams(fn, args, extendedEnv)

		evaluated := Eval(fn.Body, extendedEnv)

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
	bindParams(fn, args, env)
	return env
}

// bindParams связывает аргументы с параметрами функции, учитывая
// default-значения и rest-параметр (...args).
func bindParams(fn *Function, args []Object, env *Environment) {
	if fn.HasRest {
		fixedCount := len(fn.Parameters) - 1
		for i := 0; i < fixedCount; i++ {
			if i < len(args) {
				env.Set(fn.Parameters[i].Value, args[i])
			} else if i < len(fn.Defaults) && fn.Defaults[i] != nil {
				env.Set(fn.Parameters[i].Value, Eval(fn.Defaults[i], fn.Env))
			} else {
				env.Set(fn.Parameters[i].Value, NULL)
			}
		}
		restElems := []Object{}
		if len(args) > fixedCount {
			restElems = append(restElems, args[fixedCount:]...)
		}
		env.Set(fn.Parameters[fixedCount].Value, &Array{Elements: restElems})
	} else {
		for i, param := range fn.Parameters {
			if i < len(args) {
				env.Set(param.Value, args[i])
			} else if i < len(fn.Defaults) && fn.Defaults[i] != nil {
				env.Set(param.Value, Eval(fn.Defaults[i], fn.Env))
			} else {
				env.Set(param.Value, NULL)
			}
		}
	}
}

func unwrapReturnValue(obj Object) Object {
	if returnValue, ok := obj.(*ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

// applyFunctionFromBuiltin — упрощённая версия applyFunction, которую
// вызывают встроенные функции вроде фильтр/преобразовать.
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

// === Поддержка генераторов ===

// containsYield проверяет, есть ли в теле функции 'выдать' — это
// признак, что функция должна быть генератором.
func containsYield(node ast.Node) bool {
	if node == nil {
		return false
	}
	switch n := node.(type) {
	case *ast.YieldStatement:
		return n != nil
	case *ast.BlockStatement:
		if n == nil {
			return false
		}
		for _, stmt := range n.Statements {
			if containsYield(stmt) {
				return true
			}
		}
	case *ast.ExpressionStatement:
		if n == nil {
			return false
		}
		return containsYield(n.Expression)
	case *ast.IfExpression:
		if n == nil {
			return false
		}
		if containsYield(n.Consequence) {
			return true
		}
		if n.Alternative != nil && containsYield(n.Alternative) {
			return true
		}
	case *ast.ForExpression:
		if n == nil {
			return false
		}
		return containsYield(n.Body)
	case *ast.ForInExpression:
		if n == nil {
			return false
		}
		return containsYield(n.Body)
	case *ast.WhileExpression:
		if n == nil {
			return false
		}
		return containsYield(n.Body)
	case *ast.TryExpression:
		if n == nil {
			return false
		}
		if containsYield(n.Body) {
			return true
		}
		if n.CatchBody != nil && containsYield(n.CatchBody) {
			return true
		}
		if n.FinallyBody != nil && containsYield(n.FinallyBody) {
			return true
		}
	}
	return false
}

// GenChannel — обёртка канала генератора для хранения в Environment.
type GenChannel struct {
	Ch   chan Object
	Done chan bool
}

func (c *GenChannel) Type() string    { return "GEN_CHANNEL" }
func (c *GenChannel) Inspect() string { return "<канал генератора>" }

// generatorStopSignal — служебный сигнал прерывания генератора при
// закрытии итератора.
type generatorStopSignal struct{}

func (s *generatorStopSignal) Type() string    { return "GEN_STOP" }
func (s *generatorStopSignal) Inspect() string { return "<стоп генератора>" }

// evalPipeExpression выполняет 'x |> f(a, b)' как 'f(x, a, b)'.
// Если правая часть не вызов функции — выполняет 'right(x)'.
func evalPipeExpression(node *ast.PipeExpression, env *Environment) Object {
	left := Eval(node.Left, env)
	if isError(left) {
		return left
	}

	if call, ok := node.Right.(*ast.CallExpression); ok {
		fn := Eval(call.Function, env)
		if isError(fn) {
			return fn
		}
		args := []Object{left}
		for _, a := range call.Arguments {
			arg := Eval(a, env)
			if isError(arg) {
				return arg
			}
			args = append(args, arg)
		}
		return applyFunction(fn, args, node.Token, env)
	}

	fn := Eval(node.Right, env)
	if isError(fn) {
		return fn
	}
	return applyFunction(fn, []Object{left}, node.Token, env)
}

// evalDecoratedFunction разворачивает @a @b f в a(b(f)) и связывает
// результат с именем функции в текущем окружении.
func evalDecoratedFunction(node *ast.DecoratedFunctionStatement, env *Environment) Object {
	var value Object = Eval(node.Function, env)
	if isError(value) {
		return value
	}

	for i := len(node.Decorators) - 1; i >= 0; i-- {
		dec := Eval(node.Decorators[i], env)
		if isError(dec) {
			return dec
		}
		value = applyFunction(dec, []Object{value}, node.Token, env)
		if isError(value) {
			return value
		}
	}

	if node.Function.Name != nil {
		env.Set(node.Function.Name.Value, value)
	}
	return value
}
