package interpreter

import (
	"bufio"
	"fmt"
	"os"
)

// Ввод и вывод.

func init() {
	builtins["вывод"] = &Builtin{
		Fn: func(args ...Object) Object {
			for _, arg := range args {
				fmt.Fprint(OutputWriter, arg.Inspect())
			}
			fmt.Fprintln(OutputWriter)
			return NULL
		},
	}
	builtins["ввод"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) > 0 {
				fmt.Print(args[0].Inspect())
			}
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				return &String{Value: scanner.Text()}
			}
			return NULL
		},
	}
}
