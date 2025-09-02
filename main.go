package main

import (
	"os"

	"hadydotai/opdlang/logging"

	"github.com/jessevdk/go-flags"
)

type Options struct {
	LogLevel logging.LogLevel `short:"l" long:"loglevel" description:"Set the level of logging" choice:"none" choice:"info" choice:"debug" default:"info"`
}

var (
	opts        Options
	flagsparser = flags.NewParser(&opts, flags.Default)
)

func main() {
	flagsparser.CommandHandler = func(command flags.Commander, args []string) error {
		logging.Setup(opts.LogLevel)
		return command.Execute(args)
	}

	if _, err := flagsparser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		case flags.ErrorType:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			os.Exit(1)
		default:
			os.Exit(1)
		}
	}

	logging.Setup(opts.LogLevel)
}

// func runCompiledFile(filename string, debug bool) int {
// 	bytecode, err := os.ReadFile(filename)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
// 		return exitError
// 	}

// 	vm := NewVM(bytecode, 1024, 1024, debug)
// 	RegisterBuiltins(vm)

// 	// Start VM execution
// 	vm.Run()
// 	<-vm.stateChan // Wait for completion

// 	return exitSuccess
// }

// func runSourceFile(filename string, debug bool) int {
// 	source, err := os.ReadFile(filename)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
// 		return exitError
// 	}

// 	if debug {
// 		return startREPL(string(source))
// 	}

// 	parser := participle.MustBuild[Program](
// 		participle.Lexer(basicLexer),
// 	)

// 	program, err := parser.ParseString(filename, string(source))
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
// 		return exitError
// 	}

// 	compiler := NewCompiler()
// 	bytecode, err := compiler.compileProgram(program)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
// 		return exitError
// 	}

// 	// compiler.DebugPrint() // This will show us the compiled bytecode

// 	vm := NewVM(bytecode, 1024, 1024, debug)
// 	RegisterBuiltins(vm)

// 	// Register source map and strings
// 	for pc, line := range compiler.GetSourceMap() {
// 		vm.RegisterSourceMap(pc, line)
// 	}
// 	vm.RegisterStrings(compiler.strings)

// 	vm.Run()
// 	// Wait for final state (after all operations complete)
// 	<-vm.stateChan

// 	return exitSuccess
// }

// func startREPL(initialSource string) int {
// 	var program *Program
// 	var err error

// 	if initialSource != "" {
// 		parser := participle.MustBuild[Program](participle.Lexer(basicLexer))
// 		program, err = parser.ParseString("", initialSource)
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
// 			return exitError
// 		}
// 	}

// 	compiler := NewCompiler()
// 	var bytecode []byte
// 	if program != nil {
// 		bytecode, err = compiler.compileProgram(program)
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
// 			return exitError
// 		}
// 	}

// 	vm := NewVM(bytecode, 1024, 1024, true)
// 	RegisterBuiltins(vm)

// 	// Register source map from compiler
// 	for pc, line := range compiler.GetSourceMap() {
// 		vm.RegisterSourceMap(pc, line)
// 	}

// 	// Register the strings from the compiler
// 	vm.RegisterStrings(compiler.strings)

// 	// Set initial breakpoint at PC=0 if we have source
// 	if initialSource != "" {
// 		vm.SetLineBreakpoint(1, true)
// 	}

// 	repl := NewREPL(vm, compiler)
// 	repl.Start()
// 	return exitSuccess
// }

// func compileSource(source string, inputFile string) ([]byte, error) {
// 	parser := participle.MustBuild[Program](
// 		participle.Lexer(basicLexer),
// 	)

// 	program, err := parser.ParseString(inputFile, source)
// 	if err != nil {
// 		return nil, fmt.Errorf("parse error: %v", err)
// 	}

// 	compiler := NewCompiler()
// 	bytecode, err := compiler.compileProgram(program)
// 	if err != nil {
// 		return nil, fmt.Errorf("compilation error: %v", err)
// 	}
// 	return bytecode, nil
// }

// // func setupVM(bytecode []byte) *VM {
// // 	vm := NewVM(bytecode, 1024, 1024)
// // 	RegisterBuiltins(vm)

// // 	// Create a new compiler to handle strings and source maps
// // 	compiler := NewCompiler()
// // 	compiler
// // 	compiler.bytecode = bytecode

// // 	// Register source map from compiler
// // 	for pc, line := range compiler.GetSourceMap() {
// // 		vm.RegisterSourceMap(pc, line)
// // 	}

// // 	// Register the strings from the compiler
// // 	vm.RegisterStrings(compiler.strings)

// // 	return vm
// }
