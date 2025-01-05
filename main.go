package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/alecthomas/participle/v2"
)

const exampleSource2 = `val i = 0
print(i + 1, "i + 1")
`

func main() {
	// Parse and compile the source
	sourceCode := strings.TrimSpace(exampleSource2) // Store the source code
	parser := participle.MustBuild[Program](
		participle.Lexer(basicLexer),
	)

	program, err := parser.ParseString("", sourceCode)
	if err != nil {
		log.Fatal(err)
	}

	compiler := NewCompiler()
	bytecode := compiler.compileProgram(program)
	// compiler.DebugPrint()

	// Create VM and register built-in functions
	vm := NewVM(bytecode, 1024, 1024)

	// Register source map from compiler
	for pc, line := range compiler.GetSourceMap() {
		vm.RegisterSourceMap(pc, line)
	}

	// Register the strings from the compiler
	vm.RegisterStrings(compiler.strings)

	// Register the print function
	vm.RegisterFunction(0, func(args []Value) Value {
		for i, arg := range args {
			if i > 0 {
				fmt.Print(" ")
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

	// Set initial breakpoint at PC=0
	vm.SetLineBreakpoint(1, true)

	// Create REPL and start it, passing the source code
	repl := NewREPL(vm, compiler)
	repl.sourceCode = sourceCode // Add this field to REPL struct
	repl.Start()
}
