package interpreter

import (
	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// evalNewExpression создаёт экземпляр класса: новый Класс(args).
// В Ясном вызов класса как функции (Класс(args)) тоже создаёт
// экземпляр — оба пути обрабатываются единообразно через
// applyFunction для случая Hash.
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

	// Конструктор: 'инициализация' или его синоним 'создать'.
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

// evalInstanceIndexExpression — доступ к свойству или методу
// экземпляра. Сначала ищется в собственных свойствах, потом в методах
// класса, потом в методах родителя.
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

	if val, ok := inst.Properties[key]; ok {
		return val
	}

	keyObj := &String{Value: key}
	if pair, ok := inst.Class.Pairs[keyObj.HashKey()]; ok {
		return pair.Value
	}

	if inst.Parent != nil {
		if pair, ok := inst.Parent.Pairs[keyObj.HashKey()]; ok {
			return pair.Value
		}
	}

	return NULL
}
