package interpreter

import (
	"fmt"
	"strconv"
	"yasny-lang/ast"
)

// Object - базовый интерфейс для всех объектов
type Object interface {
	Type() string
	Inspect() string
}

// Integer - целое число
type Integer struct {
	Value int64
}

func (i *Integer) Type() string      { return "INTEGER" }
func (i *Integer) Inspect() string   { return fmt.Sprintf("%d", i.Value) }

// Float - дробное число
type Float struct {
	Value float64
}

func (f *Float) Type() string      { return "FLOAT" }
func (f *Float) Inspect() string {
	// Используем %g для красивого вывода без лишних нулей
	s := strconv.FormatFloat(f.Value, 'g', -1, 64)
	// Гарантируем, что есть точка или 'e' (отличаем от целого)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == 'e' || s[i] == 'E' {
			return s
		}
	}
	return s + ".0"
}

// String - строка
type String struct {
	Value string
}

func (s *String) Type() string      { return "STRING" }
func (s *String) Inspect() string   { return s.Value }

// Boolean - булево значение
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() string      { return "BOOLEAN" }
func (b *Boolean) Inspect() string {
	if b.Value {
		return "да"
	}
	return "нет"
}

// Null - нет/null
type Null struct{}

func (n *Null) Type() string      { return "NULL" }
func (n *Null) Inspect() string   { return "нет" }

// ReturnValue - возвращаемое значение
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() string    { return "RETURN_VALUE" }
func (rv *ReturnValue) Inspect() string { return rv.Value.Inspect() }

// Break - прервать цикл
type Break struct{}

func (b *Break) Type() string    { return "BREAK" }
func (b *Break) Inspect() string { return "прервать" }

// Continue - продолжить цикл
type Continue struct{}

func (c *Continue) Type() string    { return "CONTINUE" }
func (c *Continue) Inspect() string { return "продолжить" }

// Function - функция
type Function struct {
	Parameters  []*ast.Identifier
	Defaults    []ast.Expression
	HasRest     bool
	Body        *ast.BlockStatement
	Env         *Environment
	IsGenerator bool
}

func (f *Function) Type() string      { return "FUNCTION" }
func (f *Function) Inspect() string   { return "функция(...)" }

// Builtin - встроенная функция
type Builtin struct {
	Fn func(args ...Object) Object
}

func (b *Builtin) Type() string      { return "BUILTIN" }
func (b *Builtin) Inspect() string   { return "встроенная функция" }

// Array - массив
type Array struct {
	Elements []Object
}

func (ao *Array) Type() string { return "ARRAY" }
func (ao *Array) Inspect() string {
	result := "["
	for i, e := range ao.Elements {
		if i > 0 {
			result += ", "
		}
		result += e.Inspect()
	}
	result += "]"
	return result
}

// Error - ошибка
type Error struct {
	Message string
	Line    int
	Column  int
}

func (e *Error) Type() string { return "ERROR" }
func (e *Error) Inspect() string {
	return e.Message
}

// ErrorValue - значение ошибки (не пропагируется как ошибка)
type ErrorValue struct {
	Message string
}

func (e *ErrorValue) Type() string    { return "ERROR_VALUE" }
func (e *ErrorValue) Inspect() string { return e.Message }

// HashKey - ключ для хеш-таблицы
type HashKey struct {
	Type  string
	Value uint64
}

// Hashable - интерфейс для объектов, которые могут быть ключами
type Hashable interface {
	HashKey() HashKey
}

func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

func (b *Boolean) HashKey() HashKey {
	var value uint64
	if b.Value {
		value = 1
	} else {
		value = 0
	}
	return HashKey{Type: b.Type(), Value: value}
}

func (s *String) HashKey() HashKey {
	// Простой хеш для строк
	var hash uint64
	for _, ch := range s.Value {
		hash = hash*31 + uint64(ch)
	}
	return HashKey{Type: s.Type(), Value: hash}
}

// HashPair - пара ключ-значение
type HashPair struct {
	Key   Object
	Value Object
}

// Hash - словарь
type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() string { return "HASH" }
func (h *Hash) Inspect() string {
	result := "{"
	i := 0
	for _, pair := range h.Pairs {
		if i > 0 {
			result += ", "
		}
		result += pair.Key.Inspect() + ": " + pair.Value.Inspect()
		i++
	}
	result += "}"
	return result
}

// Instance - экземпляр класса
type Instance struct {
	Class      *Hash
	Parent     *Hash  // Родительский класс для наследования
	ParentName string // Имя родительского класса (для ленивой загрузки)
	Properties map[string]Object
}

func (inst *Instance) Type() string { return "INSTANCE" }
func (inst *Instance) Inspect() string {
	return fmt.Sprintf("<экземпляр>")
}

// Константы
var (
	NULL  = &Null{}
	TRUE  = &Boolean{Value: true}
	FALSE = &Boolean{Value: false}
)

// Generator - ленивый генератор на основе goroutine + channel
type Generator struct {
	Ch     chan Object  // канал для значений
	Done   chan bool    // канал завершения
	closed bool
}

func (g *Generator) Type() string { return "GENERATOR" }
func (g *Generator) Inspect() string { return "<генератор>" }

// Next возвращает следующее значение и флаг "ещё есть"
func (g *Generator) Next() (Object, bool) {
	if g.closed {
		return NULL, false
	}
	val, ok := <-g.Ch
	if !ok {
		g.closed = true
		return NULL, false
	}
	return val, true
}

// Close закрывает генератор
func (g *Generator) Close() {
	if !g.closed {
		g.closed = true
		select {
		case g.Done <- true:
		default:
		}
	}
}

// Future - результат асинхронной операции
type Future struct {
	Done   chan struct{}
	Result Object
	mu     bool // помечен ли результат
}

func (f *Future) Type() string    { return "FUTURE" }
func (f *Future) Inspect() string { return "<future>" }

// Wait ожидает завершения и возвращает результат
func (f *Future) Wait() Object {
	<-f.Done
	return f.Result
}
