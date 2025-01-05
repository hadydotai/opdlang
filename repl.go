package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/chzyer/readline"
)

type REPL struct {
	vm         *VM
	compiler   *Compiler
	rl         *readline.Instance
	sourceCode string
}

func NewREPL(vm *VM, compiler *Compiler) *REPL {
	// Configure readline with nice defaults
	rlConfig := &readline.Config{
		Prompt:          "\033[32m⟩\033[0m ",
		HistoryFile:     "/tmp/.vm_debugger_history",
		HistoryLimit:    1000,
		AutoComplete:    completer{},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	}

	rl, err := readline.NewEx(rlConfig)
	if err != nil {
		panic(err)
	}

	return &REPL{
		vm:       vm,
		compiler: compiler,
		rl:       rl,
	}
}

// completer implements readline.AutoCompleter
type completer struct{}

func (c completer) Do(line []rune, pos int) (newLine [][]rune, length int) {
	commands := []string{
		"step", "s", "n",
		"back", "b",
		"continue", "c",
		"break",
		"stack",
		"locals",
		"pc",
		"restart", "r",
		"load",
		"quit", "q",
		"help", "h",
	}

	input := string(line[:pos])
	var matches []string

	for _, cmd := range commands {
		if strings.HasPrefix(cmd, input) {
			matches = append(matches, cmd)
		}
	}

	for _, match := range matches {
		newLine = append(newLine, []rune(match))
	}
	return newLine, len(input)
}

func (r *REPL) printHelp() {
	help := `
Available Commands:
  step, s, n       Execute next instruction
  back, b          Step back to previous state
  continue, c      Continue execution
  break <line>     Set breakpoint at line number
  stack            Show current stack
  locals           Show local variables
  pc               Show current program counter
  restart, r       Restart program execution
  load <file>      Load and execute a source file
  help, h          Show this help message
  quit, q          Exit debugger

Tips:
  - Use Tab for command completion
  - Use Up/Down arrows for command history
  - Ctrl+A to move to start of line
  - Ctrl+E to move to end of line
  - Ctrl+W to delete previous word
  - Ctrl+L to clear screen
`
	fmt.Println(help)
}

func (r *REPL) Start() {
	defer r.rl.Close()

	fmt.Println("\033[1;36mVM Debugger REPL v0.1\033[0m")
	fmt.Println("Type 'help' or 'h' for available commands")
	fmt.Println()

	// Start VM execution and get initial state
	r.vm.Run()
	<-r.vm.stateChan

	for {
		line, err := r.rl.Readline()
		if err != nil { // io.EOF, readline.ErrInterrupt
			break
		}

		input := strings.TrimSpace(line)
		args := strings.Fields(input)

		if len(args) == 0 {
			continue
		}

		switch args[0] {
		case "help", "h":
			r.printHelp()

		case "step", "s", "n":
			if r.vm.currentState.PC >= len(r.vm.bytecode) {
				fmt.Println("\033[31mProgram has finished execution\033[0m")
				r.restartVM()
				continue
			}
			r.vm.StepNext()
			r.printState(r.vm.State())

		case "back", "b":
			r.vm.StepBack()
			r.printState(r.vm.State())

		case "continue", "c":
			r.vm.Continue()

		case "break":
			if len(args) < 2 {
				fmt.Println("Usage: break <line>")
				continue
			}
			line, err := strconv.Atoi(args[1])
			if err != nil {
				fmt.Printf("Invalid line number: %s\n", args[1])
				continue
			}
			r.vm.SetLineBreakpoint(line, true)
			fmt.Printf("Breakpoint set at line %d\n", line)

		case "stack":
			state := r.vm.State()
			fmt.Println("Stack:", r.formatStack(state.Stack))

		case "locals":
			state := r.vm.State()
			fmt.Println("Locals:", r.formatStack(state.Locals))

		case "pc":
			state := r.vm.State()
			fmt.Printf("PC: %d (Instruction: %s)\n", state.PC, Instr(r.vm.bytecode[state.PC]))

		case "restart", "r":
			r.restartVM()
			fmt.Println("Program restarted")

		case "load":
			if len(args) < 2 {
				fmt.Println("Usage: load <filename>")
				continue
			}
			err := r.loadFile(args[1])
			if err != nil {
				fmt.Printf("\033[31mError loading file: %v\033[0m\n", err)
				continue
			}
			fmt.Printf("\033[32mLoaded file: %s\033[0m\n", args[1])
			r.printState(r.vm.State())

		case "quit", "q":
			fmt.Println("\033[32mGoodbye!\033[0m")
			return

		default:
			fmt.Printf("\033[31mUnknown command: %s\033[0m\n", args[0])
		}
	}
}

func (r *REPL) restartVM() {
	r.vm.Stop()
	newState := NewVmState(r.vm.bytecode, cap(r.vm.currentState.Stack), cap(r.vm.currentState.Locals))
	newState.Strings = make([]string, len(r.vm.currentState.Strings))
	copy(newState.Strings, r.vm.currentState.Strings)
	r.vm.currentState = newState
	r.vm.history = make([]*VMState, 0)
	r.vm.Run()
	<-r.vm.stateChan
}

func (r *REPL) printState(state *VMState) {
	if state == nil {
		fmt.Println("\033[31mProgram finished execution\033[0m")
		return
	}

	if r.vm.currentState.PC >= len(r.vm.bytecode) {
		fmt.Println("\033[31mProgram finished execution\033[0m")
		return
	}

	fmt.Printf("\033[1;34mLine %d\033[0m, \033[1;35mPC: %d\033[0m (\033[1;33mInstruction: %s\033[0m)\n",
		state.SourceLine,
		state.PC,
		Instr(r.vm.bytecode[state.PC]))
	fmt.Printf("\033[1;32mStack:\033[0m %s\n", r.formatStack(state.Stack))
	fmt.Printf("\033[1;36mLocals:\033[0m %s\n", r.formatStack(state.Locals))
}

func (r *REPL) formatStack(stack []Value) string {
	var values []string
	for _, v := range stack {
		switch val := v.(type) {
		case IntValue:
			values = append(values, fmt.Sprintf("%d", val))
		case StringValue:
			values = append(values, fmt.Sprintf("%q", r.vm.currentState.Strings[val.Index]))
		default:
			values = append(values, fmt.Sprintf("%v", v))
		}
	}
	return "[" + strings.Join(values, ", ") + "]"
}

func (r *REPL) loadFile(filename string) error {
	source, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	parser := participle.MustBuild[Program](participle.Lexer(basicLexer))
	program, err := parser.ParseString(filename, string(source))
	if err != nil {
		return fmt.Errorf("parse error: %v", err)
	}

	r.compiler = NewCompiler()
	bytecode, err := r.compiler.compileProgram(program)
	if err != nil {
		return fmt.Errorf("compilation error: %v", err)
	}

	// Create new VM with the compiled bytecode
	r.vm = NewVM(bytecode, 1024, 1024, true)
	RegisterBuiltins(r.vm)

	// Register source map from compiler
	for pc, line := range r.compiler.GetSourceMap() {
		r.vm.RegisterSourceMap(pc, line)
	}

	// Register the strings from the compiler
	r.vm.RegisterStrings(r.compiler.strings)

	// Set initial breakpoint at first line
	r.vm.SetLineBreakpoint(1, true)
	r.sourceCode = string(source)

	return nil
}
