package main

import (
	"fmt"
	"hadydotai/opdlang/lang"
	"hadydotai/opdlang/logging"
	"os"
)

type CompileCommand struct {
	Output       string `short:"o" long:"output" description:"Output file and path of the compiled bytecode file" required:"yes"`
	DumpBytecode bool   `short:"d" long:"dump" description:"Dump a visual analysis of the bytecode for inspection"`
	Run          bool   `short:"r" long:"run" description:"Run the compiled bytecode file"`
	Args         struct {
		Files []string `positional-arg-name:"FILES" required:"yes"`
	} `positional-args:"yes"`
}

var compileCommand CompileCommand

func (cmd *CompileCommand) Execute(args []string) error {
	logging.Log(logging.LogLevelInfo, "Compiling single file", "file-input", cmd.Args.Files[0], "file-output", cmd.Output)
	//TODO(@hadydotai): supporting only one input file for now
	sourceFile := cmd.Args.Files[0]
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %w", sourceFile, err)
	}

	program, err := lang.Parse(sourceFile, string(source))
	if err != nil {
		return err
	}

	logging.Log(logging.LogLevelDebug, "Compilation started")
	compiler := lang.NewCompiler()
	bytecode, err := compiler.CompileProgram(program)
	if err != nil {
		return fmt.Errorf("failed to compile source file %s: %w", sourceFile, err)
	}

	logging.Log(logging.LogLevelDebug, "Committing output to disk")
	err = os.WriteFile(cmd.Output, bytecode, 0644)
	if err != nil {
		return fmt.Errorf("failed to write compiled bytecode to disk: %w", err)
	}

	logging.Log(logging.LogLevelInfo, "Successfully compiled", "file-input", cmd.Args.Files[0], "file-output", cmd.Output)

	if cmd.Run {
		logging.Log(logging.LogLevelInfo, "Running compiled output")
		runBytecode(compiler)
	}

	if cmd.DumpBytecode {
		compiler.DebugPrint()
	}
	return nil
}

func init() {
	flagsparser.AddCommand(
		"compile",
		"Compile a program into bytecode",
		"This will accept a collection of files/modules and generate a single bytecode executable from them",
		&compileCommand,
	)
}
