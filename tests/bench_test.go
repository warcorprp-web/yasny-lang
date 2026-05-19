package tests

import (
	"os"
	"testing"

	"yasny-lang/interpreter"
	"yasny-lang/lexer"
	"yasny-lang/parser"
)

// Бенчмарки производительности для разных типов задач.
// Запуск: go test ./tests -bench=. -benchtime=3s

const fibCode = `
функция фиб(n)
    если n < 2: вернуть n
    вернуть фиб(n - 1) + фиб(n - 2)
конец

фиб(20)
`

const bubbleSortCode = `
функция сортировать(массив)
    конст n = длина(массив)
    перем i = 0
    пока i < n
        перем j = 0
        пока j < n - i - 1
            если массив[j] > массив[j + 1]
                перем t = массив[j]
                массив[j] = массив[j + 1]
                массив[j + 1] = t
            конец
            j = j + 1
        конец
        i = i + 1
    конец
    вернуть массив
конец

перем a = []
перем k = 100
пока k > 0
    добавить(a, k)
    k = k - 1
конец
сортировать(a)
`

const primesCode = `
функция простое?(n)
    если n < 2: вернуть нет
    если n < 4: вернуть да
    если n % 2 == 0: вернуть нет
    перем д = 3
    пока д * д <= n
        если n % д == 0: вернуть нет
        д = д + 2
    конец
    вернуть да
конец

перем простые = []
для i от 2 до 1000
    если простое?(i): добавить(простые, i)
конец
длина(простые)
`

const stringConcatCode = `
перем s = ""
для i от 1 до 500
    s = s + "x"
конец
длина(s)
`

const arrayOpsCode = `
перем a = []
для i от 1 до 1000
    добавить(a, i)
конец
конст удвоен = преобразовать(a, x => x * 2)
конст чётные = фильтр(удвоен, x => x % 4 == 0)
свернуть(чётные, (s, x) => s + x, 0)
`

func runYasnyCode(b *testing.B, code string) {
	b.Helper()
	// Подавляем вывод чтобы не замедлял.
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	old := interpreter.OutputWriter
	interpreter.OutputWriter = devNull
	defer func() {
		interpreter.OutputWriter = old
		devNull.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.NewWithFilename(code, "bench.ya")
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			b.Fatalf("ошибки парсинга: %v", p.Errors())
		}
		env := interpreter.NewEnvironment()
		result := interpreter.Eval(program, env)
		if result != nil && result.Type() == "ERROR" {
			b.Fatalf("ошибка выполнения: %s", result.Inspect())
		}
	}
}

func BenchmarkFib20(b *testing.B)        { runYasnyCode(b, fibCode) }
func BenchmarkBubbleSort100(b *testing.B) { runYasnyCode(b, bubbleSortCode) }
func BenchmarkPrimesUpTo1000(b *testing.B) { runYasnyCode(b, primesCode) }
func BenchmarkStringConcat500(b *testing.B) { runYasnyCode(b, stringConcatCode) }
func BenchmarkArrayOps1000(b *testing.B)    { runYasnyCode(b, arrayOpsCode) }
