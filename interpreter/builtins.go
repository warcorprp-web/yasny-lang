package interpreter

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
	"time"
)

// OutputWriter - куда писать вывод (для WASM можно переопределить)
var OutputWriter io.Writer = os.Stdout

// Счётчики тестов
var testPasses, testFailures int

var ApplyFunctionCallback func(Object, []Object) Object

// Вспомогательные функции для ошибок встроенных функций
func builtinErrorWrongArgCount(funcName string, expected, got int) *Error {
	return ErrorWithHint(
		currentCallToken,
		fmt.Sprintf("функция '%s' ожидает %d аргумент(ов), получено %d", funcName, expected, got),
		fmt.Sprintf("Вызовите функцию правильно: %s(...)", funcName),
	)
}

func builtinErrorWrongArgType(funcName string, argNum int, expected, got string) *Error {
	return ErrorWithHint(
		currentCallToken,
		fmt.Sprintf("функция '%s': аргумент %d должен быть %s, получен %s", funcName, argNum, expected, got),
		fmt.Sprintf("Передайте значение типа %s", expected),
	)
}

func builtinErrorUnsupportedType(funcName string, got string) *Error {
	return ErrorWithHint(
		currentCallToken,
		fmt.Sprintf("функция '%s' не поддерживает тип %s", funcName, got),
		"Проверьте типы аргументов.",
	)
}

// newBuiltinError создает ошибку с номером строки из текущего вызова
func newBuiltinError(format string, a ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf(format, a...),
		Line:    currentCallToken.Line,
		Column:  currentCallToken.Column,
	}
}

var builtins = make(map[string]*Builtin)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func nativeToObject(data interface{}) Object {
	switch v := data.(type) {
	case nil:
		return NULL
	case bool:
		if v {
			return TRUE
		}
		return FALSE
	case float64:
		if v == float64(int64(v)) {
			return &Integer{Value: int64(v)}
		}
		return &Float{Value: v}
	case string:
		return &String{Value: v}
	case []interface{}:
		elements := make([]Object, len(v))
		for i, elem := range v {
			elements[i] = nativeToObject(elem)
		}
		return &Array{Elements: elements}
	case map[string]interface{}:
		// Сортируем ключи, чтобы порядок при разборе JSON был
		// стабильным (Go-карта итерируется в случайном порядке).
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		h := NewHash()
		for _, k := range keys {
			h.Set(&String{Value: k}, nativeToObject(v[k]))
		}
		return h
	default:
		return NULL
	}
}

func objectToNative(obj Object) interface{} {
	switch o := obj.(type) {
	case *Integer:
		return o.Value
	case *Float:
		return o.Value
	case *String:
		return o.Value
	case *Boolean:
		return o.Value
	case *Array:
		result := make([]interface{}, len(o.Elements))
		for i, elem := range o.Elements {
			result[i] = objectToNative(elem)
		}
		return result
	case *Hash:
		result := make(map[string]interface{})
		for _, pair := range o.Pairs {
			if key, ok := pair.Key.(*String); ok {
				result[key.Value] = objectToNative(pair.Value)
			}
		}
		return result
	case *Null:
		return nil
	default:
		return nil
	}
}

func toFloat(obj Object) *float64 {
	switch o := obj.(type) {
	case *Integer:
		f := float64(o.Value)
		return &f
	case *Float:
		return &o.Value
	default:
		return nil
	}
}
