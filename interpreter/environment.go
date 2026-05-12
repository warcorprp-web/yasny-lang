package interpreter

const MaxCallDepth = 1000

// Environment - окружение для хранения переменных
type Environment struct {
	store     map[string]Object
	immutable map[string]bool // отслеживание immutable переменных
	exports   map[string]Object // экспортированные значения
	outer     *Environment
	callDepth int // глубина вызовов для этого окружения
}

// NewEnvironment создает новое окружение
func NewEnvironment() *Environment {
	s := make(map[string]Object)
	i := make(map[string]bool)
	e := make(map[string]Object)
	return &Environment{store: s, immutable: i, exports: e, outer: nil}
}

// NewEnclosedEnvironment создает вложенное окружение
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	env.callDepth = outer.callDepth
	return env
}

// Get получает значение переменной
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

// Set устанавливает значение переменной
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

// Update обновляет существующую переменную (ищет в родительских scope)
func (e *Environment) Update(name string, val Object) (Object, bool) {
	// Ищем переменную в текущем scope
	if _, ok := e.store[name]; ok {
		e.store[name] = val
		return val, true
	}
	// Ищем в родительском scope
	if e.outer != nil {
		return e.outer.Update(name, val)
	}
	return nil, false
}

// SetImmutable устанавливает immutable переменную
func (e *Environment) SetImmutable(name string, val Object) Object {
	e.store[name] = val
	e.immutable[name] = true
	return val
}

// IsImmutable проверяет является ли переменная immutable
func (e *Environment) IsImmutable(name string) bool {
	if e.immutable[name] {
		return true
	}
	if e.outer != nil {
		return e.outer.IsImmutable(name)
	}
	return false
}

// Export помечает переменную как экспортированную
func (e *Environment) Export(name string) {
	if val, ok := e.store[name]; ok {
		e.exports[name] = val
	}
}
