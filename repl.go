package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type REPL struct {
	vm         *VM
	compiler   *Compiler
	reader     *bufio.Reader
	sourceCode string
}

func NewREPL(vm *VM, compiler *Compiler) *REPL {
	return &REPL{
		vm:       vm,
		compiler: compiler,
		reader:   bufio.NewReader(os.Stdin),
	}
}

func (r *REPL) Start() {
	fmt.Println("VM Debugger REPL")
	fmt.Println("Available commands:")
	fmt.Println("  step, s, n - Execute next instruction")
	fmt.Println("  back, b - Step back to previous state")
	fmt.Println("  continue, c - Continue execution")
	fmt.Println("  break <line> - Set breakpoint at line number")
	fmt.Println("  stack - Show current stack")
	fmt.Println("  locals - Show local variables")
	fmt.Println("  pc - Show current program counter")
	fmt.Println("  restart, r - Restart program execution")
	fmt.Println("  quit, q - Exit debugger")

	// Start VM execution and get initial state
	r.vm.Run()
	<-r.vm.stateChan

	for {
		fmt.Print("> ")
		input, _ := r.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		args := strings.Fields(input)

		if len(args) == 0 {
			continue
		}

		switch args[0] {
		case "step", "s", "n":
			if r.vm.currentState.PC >= len(r.vm.bytecode) {
				fmt.Println("Program has finished execution")
				r.vm.Stop()
				newState := NewVmState(r.vm.bytecode, cap(r.vm.currentState.Stack), cap(r.vm.currentState.Locals))
				newState.Strings = make([]string, len(r.vm.currentState.Strings))
				copy(newState.Strings, r.vm.currentState.Strings)
				r.vm.currentState = newState
				r.vm.history = make([]*VMState, 0)
				r.vm.Run()
				<-r.vm.stateChan
				continue
			}
			r.vm.StepNext()
			// r.printState(state)

		case "back", "b":
			r.vm.StepBack()
			// r.printState(state)

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
			r.vm.Stop()
			newState := NewVmState(r.vm.bytecode, cap(r.vm.currentState.Stack), cap(r.vm.currentState.Locals))
			newState.Strings = make([]string, len(r.vm.currentState.Strings))
			copy(newState.Strings, r.vm.currentState.Strings)
			r.vm.currentState = newState
			r.vm.history = make([]*VMState, 0)
			r.vm.Run()
			<-r.vm.stateChan
			fmt.Println("Program restarted")

		case "quit", "q":
			return

		default:
			fmt.Printf("Unknown command: %s\n", args[0])
		}
	}
}

func (r *REPL) printState(state *VMState) {
	if state == nil {
		fmt.Println("Program finished execution")
		return
	}

	if r.vm.currentState.PC >= len(r.vm.bytecode) {
		fmt.Println("Program finished execution")
		return
	}

	fmt.Printf("Line %d, PC: %d (Instruction: %s)\n",
		state.SourceLine,
		state.PC,
		Instr(r.vm.bytecode[state.PC]))
	fmt.Println("Stack:", r.formatStack(state.Stack))
	fmt.Println("Locals:", r.formatStack(state.Locals))
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
