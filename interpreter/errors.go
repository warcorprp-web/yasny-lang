package interpreter

import (
	"fmt"
	"yasny-lang/lexer"
)

// ErrorWithHint создает ошибку с подсказкой
func ErrorWithHint(tok lexer.Token, message string, hint string) *Error {
	fullMessage := message
	if hint != "" {
		fullMessage = fmt.Sprintf("%s\n💡 Подсказка: %s", message, hint)
	}
	return &Error{
		Message: fullMessage,
		Line:    tok.Line,
		Column:  tok.Column,
	}
}

// Типичные ошибки с подсказками

func ErrorCannotReassignConst(tok lexer.Token, varName string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("нельзя изменить переменную '%s'", varName),
		"Переменная объявлена как 'конст' (неизменяемая). Используйте 'перем' если нужно менять значение.",
	)
}

func ErrorVariableNotDeclared(tok lexer.Token, varName string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("переменная '%s' не объявлена", varName),
		"Объявите переменную с помощью 'конст' или 'перем' перед использованием.",
	)
}

func ErrorIndexOutOfRange(tok lexer.Token, index int64, length int) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("индекс %d вне диапазона (длина массива: %d)", index, length),
		fmt.Sprintf("Допустимые индексы: от 0 до %d", length-1),
	)
}

func ErrorDivisionByZero(tok lexer.Token) *Error {
	return ErrorWithHint(
		tok,
		"деление на ноль",
		"Проверьте, что делитель не равен нулю перед операцией.",
	)
}

func ErrorTypeMismatch(tok lexer.Token, left, operator, right string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("несовпадение типов: %s %s %s", left, operator, right),
		"Убедитесь, что оба операнда имеют совместимые типы.",
	)
}

func ErrorUnknownOperator(tok lexer.Token, left, operator, right string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("неизвестный оператор: %s %s %s", left, operator, right),
		"Проверьте, что оператор поддерживается для этих типов данных.",
	)
}

func ErrorWrongArgumentCount(tok lexer.Token, funcName string, expected, got int) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("функция '%s' ожидает %d аргумент(ов), получено %d", funcName, expected, got),
		fmt.Sprintf("Вызовите функцию с правильным количеством аргументов: %s(...)", funcName),
	)
}

func ErrorWrongArgumentType(tok lexer.Token, funcName string, argNum int, expected, got string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("функция '%s': аргумент %d должен быть %s, получен %s", funcName, argNum, expected, got),
		fmt.Sprintf("Передайте значение типа %s", expected),
	)
}

func ErrorIdentifierNotFound(tok lexer.Token, name string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("идентификатор не найден: %s", name),
		"Проверьте правильность написания или объявите переменную/функцию.",
	)
}

func ErrorNotCallable(tok lexer.Token, typeName string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("тип %s не является функцией", typeName),
		"Убедитесь, что вызываете функцию, а не другой тип данных.",
	)
}

func ErrorPropertyNotFound(tok lexer.Token, propName, objType string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("свойство '%s' не найдено в объекте типа %s", propName, objType),
		"Проверьте правильность имени свойства или добавьте его в объект.",
	)
}

func ErrorMethodNotFound(tok lexer.Token, methodName, className string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("метод '%s' не найден в классе '%s'", methodName, className),
		"Проверьте правильность имени метода или добавьте его в класс.",
	)
}

func ErrorInvalidDestructuring(tok lexer.Token, expected, got string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("деструктуризация %s требует %s, получено %s", expected, expected, got),
		"Убедитесь, что деструктурируете правильный тип данных.",
	)
}

func ErrorSpreadNotArray(tok lexer.Token, got string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("spread оператор (...) можно применять только к массивам, получено %s", got),
		"Используйте spread только с массивами: [...массив]",
	)
}

func ErrorFileNotFound(tok lexer.Token, filename string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("файл не найден: %s", filename),
		"Проверьте путь к файлу и убедитесь, что файл существует.",
	)
}

func ErrorInvalidJSON(tok lexer.Token) *Error {
	return ErrorWithHint(
		tok,
		"неверный формат JSON",
		"Проверьте синтаксис JSON: правильные кавычки, запятые, скобки.",
	)
}

func ErrorInvalidRegex(tok lexer.Token, pattern string) *Error {
	return ErrorWithHint(
		tok,
		fmt.Sprintf("неверное регулярное выражение: %s", pattern),
		"Проверьте синтаксис регулярного выражения.",
	)
}
