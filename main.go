package main

import (
	"fmt"
	"log"

	"github.com/alecthomas/participle/v2"
)

const exampleSource = `
val i = 0
while i < 10 do
	val i = i + 1
	print(i)
end
`

func registerBuiltins(vm *VM) {
	// Register functions using the same indices defined in builtinFunctions
	vm.RegisterFunction(builtinFunctions["print"], func(args []int) int {
		for _, arg := range args {
			fmt.Printf("%d ", arg)
		}
		fmt.Println()
		return 0
	})

	vm.RegisterFunction(builtinFunctions["add"], func(args []int) int {
		if len(args) != 2 {
			return 0
		}
		return args[0] + args[1]
	})
}

func main() {
	parser, err := participle.Build[Program](
		participle.Lexer(basicLexer),
		participle.Elide("whitespace"),
		participle.Elide("comment"),
		participle.UseLookahead(2),
	)
	if err != nil {
		log.Fatalf("failed to build parser: %v", err)
	}

	program, err := parser.ParseString("", exampleSource)
	if err != nil {
		log.Fatalf("failed to parse program: %v", err)
	}

	fmt.Println("AST:")
	fmt.Printf("%+v\n", program)

	compiler := NewCompiler()
	bytecode := compiler.compileProgram(program)

	fmt.Println("\nCompiled bytecode:")
	compiler.DebugPrint()

	vm := NewVM(bytecode, 1024, 1024)
	registerBuiltins(vm)

	fmt.Println("\nExecuting program:")
	vm.RunBlock()
}
