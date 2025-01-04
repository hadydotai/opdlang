package main

import (
	"fmt"
	"log"

	"github.com/alecthomas/participle/v2"
)

const exampleSource = `
val i = 0
val j = 0
while i < 10 do
	while j < 10 do
		print(i + j)
		val j = j + 1
	end
	val i = i + 1
end
`

func main() {
	// Parse and compile the source
	parser := participle.MustBuild[Program](
		participle.Lexer(basicLexer),
	)

	program, err := parser.ParseString("", exampleSource)
	if err != nil {
		log.Fatal(err)
	}

	compiler := NewCompiler()
	bytecode := compiler.compileProgram(program)
	compiler.DebugPrint()

	// Create VM and register built-in functions
	vm := NewVM(bytecode, 1024, 1024)

	// Register the print function
	vm.RegisterFunction(builtinFunctions["print"], func(args []int) int {
		for _, arg := range args {
			fmt.Printf("%d ", arg)
		}
		fmt.Println()
		return 0
	})

	// Run the program
	vm.RunBlock()
}
