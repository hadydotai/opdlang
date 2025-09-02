package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/participle/v2"
)

type CompileCommand struct {
	Output string `short:"o" long:"output" description:"Output file and path of the compiled bytecode file" required:"yes"`
	Run    bool   `short:"r" long:"run" description:"Run the compiled bytecode file"`
	Args   struct {
		Files []string `positional-arg-name:"FILES" required:"yes"`
	} `positional-args:"yes"`
}

var compileCommand CompileCommand

func (cmd *CompileCommand) Execute(args []string) error {
	log(LogLevelInfo, "Compiling single file", "file-input", cmd.Args.Files[0], "file-output", cmd.Output)
	bytes, err := cmd.compileFile()
	if err != nil {
		return err
	}

	log(LogLevelDebug, "Committing output to disk")
	err = os.WriteFile(cmd.Output, bytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write compiled bytecode to disk: %w", err)
	}

	log(LogLevelInfo, "Successfully Compiled", "file-input", cmd.Args.Files[0], "file-output", cmd.Output)
	return nil
}

func (cmd *CompileCommand) compileFile() ([]byte, error) {
	log(LogLevelDebug, "Reading file")
	//TODO(@hadydotai): supporting only one input file for now
	sourceFile := cmd.Args.Files[0]
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file %s: %w", sourceFile, err)
	}

	parser := participle.MustBuild[Program](
		participle.Lexer(basicLexer),
	)

	log(LogLevelDebug, "Parsing source")
	program, err := parser.ParseString(sourceFile, string(source))
	if err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	log(LogLevelDebug, "Compilation started")
	compiler := NewCompiler()
	bytecode, err := compiler.compileProgram(program)
	if err != nil {
		return nil, fmt.Errorf("failed to compile source file %s: %w", sourceFile, err)
	}

	return bytecode, nil
}

func init() {
	flagsparser.AddCommand(
		"compile",
		"Compile a program into bytecode",
		"This will accept a collection of files/modules and generate a single bytecode executable from them",
		&compileCommand,
	)
}
