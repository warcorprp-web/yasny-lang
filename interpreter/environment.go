package interpreter

const MaxCallDepth = 1000

// inlineCapacity — сколько переменных хранить в inline-массиве
// перед переключением на map. Подобрано так, чтобы покрыть типовые
// функции (1–3 параметра + пара локальных переменных).
const inlineCapacity = 8

// Environment — окружение для хранения переменных.
//
// Оптимизация: маленькие окружения (до inlineCapacity переменных)
// хранятся в плоских массивах с линейным поиском, без аллокации map.
// Это типичный случай — большинство тел функций имеют 1–4 параметра.
// При превышении inline-ёмкости массив переливается в map.
//
// Корневое окружение (NewEnvironment) сразу использует map, потому
// что в нём обычно много глобальных имён.
type Environment struct {
	// Inline-хранилище для маленьких scope.
	inlineNames [inlineCapacity]string
	inlineVals  [inlineCapacity]Object
	inlineCount int

	// Запасное хранилище — map для больших scope.
	store map[string]Object

	// Иммутабельность отслеживается отдельно. Обычно очень мало
	// immutable-имён, lazy-инициализация map — оправдана.
	immutable map[string]bool

	// Экспорты для механизма модулей.
	exports map[string]Object

	outer     *Environment
	callDepth int
}

// NewEnvironment создаёт корневое окружение. Сразу использует map,
// потому что глобальный scope обычно большой.
func NewEnvironment() *Environment {
	return &Environment{
		store: make(map[string]Object, 16),
	}
}

// NewEnclosedEnvironment создаёт вложенное окружение с inline-
// хранилищем. Map выделится только если переменных станет больше
// inlineCapacity.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	return &Environment{
		outer:     outer,
		callDepth: outer.callDepth,
	}
}

// Get получает значение переменной, поднимаясь вверх по цепочке
// окружений. Сначала ищет в inline-массиве (быстро для маленьких
// scope), потом в map (если есть), потом во внешнем окружении.
func (e *Environment) Get(name string) (Object, bool) {
	// Линейный поиск по inline — для типичного scope с 1–3
	// переменными быстрее любого map.
	for i := 0; i < e.inlineCount; i++ {
		if e.inlineNames[i] == name {
			return e.inlineVals[i], true
		}
	}
	if e.store != nil {
		if obj, ok := e.store[name]; ok {
			return obj, true
		}
	}
	if e.outer != nil {
		return e.outer.Get(name)
	}
	return nil, false
}

// Set устанавливает переменную в текущем scope. Если есть место в
// inline — пишет туда, иначе в map.
func (e *Environment) Set(name string, val Object) Object {
	// Если уже в inline — обновляем.
	for i := 0; i < e.inlineCount; i++ {
		if e.inlineNames[i] == name {
			e.inlineVals[i] = val
			return val
		}
	}
	// Если уже в map — обновляем.
	if e.store != nil {
		if _, ok := e.store[name]; ok {
			e.store[name] = val
			return val
		}
	}
	// Новая переменная: пытаемся в inline.
	if e.inlineCount < inlineCapacity && e.store == nil {
		e.inlineNames[e.inlineCount] = name
		e.inlineVals[e.inlineCount] = val
		e.inlineCount++
		return val
	}
	// Иначе — переливаем inline в map и пишем туда.
	if e.store == nil {
		e.store = make(map[string]Object, 8)
		for i := 0; i < e.inlineCount; i++ {
			e.store[e.inlineNames[i]] = e.inlineVals[i]
		}
		e.inlineCount = 0
	}
	e.store[name] = val
	return val
}

// Update обновляет существующую переменную в первом окружении,
// где она найдена.
func (e *Environment) Update(name string, val Object) (Object, bool) {
	for i := 0; i < e.inlineCount; i++ {
		if e.inlineNames[i] == name {
			e.inlineVals[i] = val
			return val, true
		}
	}
	if e.store != nil {
		if _, ok := e.store[name]; ok {
			e.store[name] = val
			return val, true
		}
	}
	if e.outer != nil {
		return e.outer.Update(name, val)
	}
	return nil, false
}

// SetImmutable устанавливает immutable-переменную (объявленную через конст).
func (e *Environment) SetImmutable(name string, val Object) Object {
	e.Set(name, val)
	if e.immutable == nil {
		e.immutable = make(map[string]bool, 4)
	}
	e.immutable[name] = true
	return val
}

// IsImmutable проверяет, объявлена ли переменная как immutable.
func (e *Environment) IsImmutable(name string) bool {
	if e.immutable != nil && e.immutable[name] {
		return true
	}
	if e.outer != nil {
		return e.outer.IsImmutable(name)
	}
	return false
}

// Export помечает переменную как экспортированную для модулей.
func (e *Environment) Export(name string) {
	val, ok := e.localGet(name)
	if !ok {
		return
	}
	if e.exports == nil {
		e.exports = make(map[string]Object, 4)
	}
	e.exports[name] = val
}

// localGet ищет переменную только в этом scope (без подъёма вверх).
func (e *Environment) localGet(name string) (Object, bool) {
	for i := 0; i < e.inlineCount; i++ {
		if e.inlineNames[i] == name {
			return e.inlineVals[i], true
		}
	}
	if e.store != nil {
		if obj, ok := e.store[name]; ok {
			return obj, true
		}
	}
	return nil, false
}
