package main

import (
	"fmt"
)

type RunCommand struct {
	Args struct {
		ExecutableFile string `positional-arg-name:"EXE-FILE" required:"yes"`
	} `positional-args:"yes"`
}

var runCommand RunCommand

func (cmd *RunCommand) Execute(args []string) error {
	return fmt.Errorf("running an executable bytecode directly is not fully implemented, please use `compile` subcommand with `--run` flag")
	// bytecode, err := os.ReadFile(cmd.Args.ExecutableFile)
	// if err != nil {
	// 	return fmt.Errorf("failed to read executable bytecode file%s: %w", cmd.Args.ExecutableFile, err)
	// }

	// compiler := NewCompiler()
	//TODO(@hadydotai): Right, this obviously fails because we register information during AST->bytecode
	// which is missing from the disk written bytecode, like string interning, constant folding, all of this
	// is constructed at the AST->bytecode stage. Which means all the compiler state is zeroed out, which means
	// some of the VM calls below in `runBytecode` like `RegisterStrings` or `RegisterSourceMap` fails because the compiler
	// has none of these things available being constructed directly from the bytecode.
	// I can't remember why I did it this way, but I reckon it must have been me not wanting to deal with clearly defining the bytecode
	// format beyond what was necessary to get the debugger up and running.
	// compiler.code = bytecode
	// runBytecode(compiler)
	// return nil
}

func runBytecode(compiler *Compiler) {
	vm := NewVM(compiler.code, 1024, 1024, false)
	RegisterBuiltins(vm)

	// Register source map and strings
	for pc, line := range compiler.GetSourceMap() {
		vm.RegisterSourceMap(pc, line)
	}
	vm.RegisterStrings(compiler.strings)

	vm.Run()
	// Wait for final state (after all operations complete)
	<-vm.stateChan
}

func init() {
	flagsparser.AddCommand(
		"run",
		"Run a bytecode compiled program",
		"This will execute a bytecode compiled program (compiled with `compile` subcommand)",
		&runCommand,
	)
}
