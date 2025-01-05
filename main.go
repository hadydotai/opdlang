package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2"
)

const (
	exitSuccess = 0
	exitError   = 1
)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	// Command line flags
	var (
		compile = flag.Bool("compile", false, "Compile source to bytecode file")
		run     = flag.Bool("run", false, "Run the compiled bytecode file")
		debug   = flag.Bool("debug", false, "Start in debug mode")
		output  = flag.String("o", "", "Output file for compiled bytecode")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [filename]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Without arguments, starts an interactive REPL\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// No arguments - start REPL
	if flag.NArg() == 0 && !*compile && !*run {
		return startREPL("")
	}

	// Get the input file
	inputFile := flag.Arg(0)
	if inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: input file required")
		flag.Usage()
		return exitError
	}

	// Determine output file if not specified
	if *compile && *output == "" {
		*output = strings.TrimSuffix(inputFile, filepath.Ext(inputFile)) + ".bc"
	}

	// Handle different modes
	switch {
	case *compile:
		return compileFile(inputFile, *output)
	case *run:
		return runCompiledFile(inputFile, *debug)
	default:
		return runSourceFile(inputFile, *debug)
	}
}

func compileFile(inputFile, outputFile string) int {
	source, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return exitError
	}

	bytecode, err := compileSource(string(source), inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		return exitError
	}

	err = os.WriteFile(outputFile, []byte(bytecode), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing bytecode: %v\n", err)
		return exitError
	}

	return exitSuccess
}

func runCompiledFile(filename string, debug bool) int {
	bytecode, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return exitError
	}

	if debug {
		fmt.Fprintf(os.Stderr, "Debug mode not supported for compiled files yet\n")
		return exitError
	}

	vm := NewVM(bytecode, 1024, 1024, debug)
	RegisterBuiltins(vm)

	// Start VM execution
	vm.Run()
	<-vm.stateChan // Wait for completion

	return exitSuccess
}

func runSourceFile(filename string, debug bool) int {
	source, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		return exitError
	}

	if debug {
		return startREPL(string(source))
	}

	parser := participle.MustBuild[Program](
		participle.Lexer(basicLexer),
	)

	program, err := parser.ParseString(filename, string(source))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		return exitError
	}

	compiler := NewCompiler()
	bytecode, err := compiler.compileProgram(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		return exitError
	}

	// compiler.DebugPrint() // This will show us the compiled bytecode

	vm := NewVM(bytecode, 1024, 1024, debug)
	RegisterBuiltins(vm)

	// Register source map and strings
	for pc, line := range compiler.GetSourceMap() {
		vm.RegisterSourceMap(pc, line)
	}
	vm.RegisterStrings(compiler.strings)

	vm.Run()
	// Wait for final state (after all operations complete)
	<-vm.stateChan

	return exitSuccess
}

func startREPL(initialSource string) int {
	var program *Program
	var err error

	if initialSource != "" {
		parser := participle.MustBuild[Program](participle.Lexer(basicLexer))
		program, err = parser.ParseString("", initialSource)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
			return exitError
		}
	}

	compiler := NewCompiler()
	var bytecode []byte
	if program != nil {
		bytecode, err = compiler.compileProgram(program)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
			return exitError
		}
	}

	vm := NewVM(bytecode, 1024, 1024, true)
	RegisterBuiltins(vm)

	// Register source map from compiler
	for pc, line := range compiler.GetSourceMap() {
		vm.RegisterSourceMap(pc, line)
	}

	// Register the strings from the compiler
	vm.RegisterStrings(compiler.strings)

	// Set initial breakpoint at PC=0 if we have source
	if initialSource != "" {
		vm.SetLineBreakpoint(1, true)
	}

	repl := NewREPL(vm, compiler)
	repl.Start()
	return exitSuccess
}

func compileSource(source string, inputFile string) ([]byte, error) {
	parser := participle.MustBuild[Program](
		participle.Lexer(basicLexer),
	)

	program, err := parser.ParseString(inputFile, source)
	if err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	compiler := NewCompiler()
	bytecode, err := compiler.compileProgram(program)
	if err != nil {
		return nil, fmt.Errorf("compilation error: %v", err)
	}
	return bytecode, nil
}

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
