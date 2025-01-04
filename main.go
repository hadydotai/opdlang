package main

import (
	"fmt"
	"log"

	"github.com/alecthomas/participle/v2"
)

const exampleSource = `
val i = 0
while i < 10 do
	print(i, " outter!")
	print("----")
	val j  = 0
	while j < 10 do
		print(j + i, " inner!")
		val j = j + 1
	end
	print("----")
	val i = i + 1
end
`
const exampleSource2 = `
val i = 0
while i < 1 do
	print(i, "i")
	print("+++")
	print(i + 1, "i + 1")
	val i = i + 1
end
`

func main() {
	// Parse and compile the source
	parser := participle.MustBuild[Program](
		participle.Lexer(basicLexer),
	)

	program, err := parser.ParseString("", exampleSource2)
	if err != nil {
		log.Fatal(err)
	}

	compiler := NewCompiler()
	bytecode := compiler.compileProgram(program)
	compiler.DebugPrint()

	// Create VM and register built-in functions
	vm := NewVM(bytecode, 1024, 1024)

	// Register the strings from the compiler
	vm.RegisterStrings(compiler.strings)

	// Register the print function
	vm.RegisterFunction(0, func(args []Value) Value {
		for i, arg := range args {
			if i > 0 {
				fmt.Print(" ") // Add space between arguments
			}
			switch v := arg.(type) {
			case IntValue:
				fmt.Print(int(v))
			case StringValue:
				fmt.Print(vm.currentState.Strings[v.Index])
			}
		}
		fmt.Println()
		return IntValue(0)
	})

	// Run the program
	vm.RunBlock()
}
