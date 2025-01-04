package main

import (
	"fmt"
	"sync"
)

type DebuggerCmd int

const (
	DebuggerCmdContinue DebuggerCmd = iota
	DebuggerCmdStepNext
	DebuggerCmdStepBack
	DebuggerCmdPause
)

type VMState struct {
	PC          int
	Stack       []int
	Locals      []int
	Memory      []byte
	CallStack   []int
	ReturnStack []int
}

func (vm *VMState) Clone() *VMState {
	newState := &VMState{
		PC:          vm.PC,
		Stack:       make([]int, len(vm.Stack)),
		Locals:      make([]int, len(vm.Locals)),
		Memory:      make([]byte, len(vm.Memory)),
		CallStack:   make([]int, len(vm.CallStack)),
		ReturnStack: make([]int, len(vm.ReturnStack)),
	}
	copy(newState.Stack, vm.Stack)
	copy(newState.Locals, vm.Locals)
	copy(newState.Memory, vm.Memory)
	copy(newState.CallStack, vm.CallStack)
	copy(newState.ReturnStack, vm.ReturnStack)
	return newState
}

type Instr byte

const (
	InstrPush Instr = iota
	InstrPop
	InstrAdd
	InstrSub
	InstrMul
	InstrDiv
	InstrMod
	InstrEq
	InstrNeq
	InstrLt
	InstrGt
	InstrLte
	InstrGte
	InstrLoad
	InstrStore
	InstrJmp
	InstrJmpIfZero
	InstrJmpIfNeg
	InstrJmpIfPos
	InstrCall
	InstrRet
	InstrHalt
)

func (instr Instr) String() string {
	names := []string{
		"PUSH", "POP", "ADD", "SUB", "MUL", "DIV", "MOD",
		"EQ", "NEQ", "LT", "GT", "LTE", "GTE", "LOAD",
		"STORE", "JMP", "JMP_IF_ZERO", "JMP_IF_NEG",
		"JMP_IF_POS", "CALL", "RET", "HALT",
	}
	if int(instr) < len(names) {
		return names[instr]
	}
	return "UNKNOWN"
}

type GoFunction func(args []int) int

type VM struct {
	currentState *VMState
	history      []*VMState
	bytecode     []byte

	debugChan chan DebuggerCmd
	stateChan chan *VMState

	mu          sync.RWMutex
	breakpoints map[int]bool
	running     bool
	functions   map[int]GoFunction
}

func NewVM(bytecode []byte, stackSize, localsSize int) *VM {
	return &VM{
		bytecode: bytecode,
		currentState: &VMState{
			Stack:  make([]int, 0, stackSize),
			Locals: make([]int, 0, localsSize),
			Memory: make([]byte, 0, 1024),
		},
		debugChan:   make(chan DebuggerCmd),
		stateChan:   make(chan *VMState),
		breakpoints: make(map[int]bool),
		history:     make([]*VMState, 0),
		running:     false,
		functions:   make(map[int]GoFunction),
	}
}

func (vm *VM) SetBreakpoint(pc int, enabled bool) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.breakpoints[pc] = enabled
	if !enabled {
		delete(vm.breakpoints, pc)
	}
}

func (vm *VM) Run() {
	vm.running = true
	go vm.execute()
}

func (vm *VM) RunBlock() {
	vm.running = true
	vm.execute()
}

func (vm *VM) RunUntil(pc int) {
	vm.running = true
	vm.currentState.PC = pc
	go vm.execute()
}

func (vm *VM) Debug(cmd DebuggerCmd) *VMState {
	vm.debugChan <- cmd
	return <-vm.stateChan
}

func (vm *VM) StepNext() *VMState {
	return vm.Debug(DebuggerCmdStepNext)
}

func (vm *VM) StepBack() *VMState {
	return vm.Debug(DebuggerCmdStepBack)
}

func (vm *VM) Pause() {
	vm.Debug(DebuggerCmdPause)
}

func (vm *VM) Continue() {
	vm.Debug(DebuggerCmdContinue)
}

func (vm *VM) State() *VMState {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.currentState.Clone()
}

func (vm *VM) execute() {
	for vm.running && vm.currentState.PC < len(vm.bytecode) {
		vm.mu.RLock()
		hasBreakpoint := vm.breakpoints[vm.currentState.PC]
		vm.mu.RUnlock()

		if hasBreakpoint {
			vm.stateChan <- vm.currentState.Clone()
			cmd := <-vm.debugChan

			switch cmd {
			case DebuggerCmdPause:
				vm.running = false
				vm.history = append(vm.history, vm.currentState.Clone())
				return
			case DebuggerCmdStepNext:
				vm.currentState.PC++
				vm.history = append(vm.history, vm.currentState.Clone())
			case DebuggerCmdStepBack:
				if len(vm.history) > 0 {
					vm.currentState = vm.history[len(vm.history)-1]
					vm.history = vm.history[:len(vm.history)-1]
				}
				continue
			case DebuggerCmdContinue:
				vm.running = true
				// vm.currentState.PC++
			}
		}

		vm.history = append(vm.history, vm.currentState.Clone())
		err := vm.executeInstruction()
		if err != nil {
			fmt.Println("Execution error:", err)
			vm.running = false
			return
		}
	}
}

func (vm *VM) executeInstruction() error {
	instruction := vm.bytecode[vm.currentState.PC]
	vm.currentState.PC++

	switch Instr(instruction) {
	case InstrPush:
		return vm.executePush()
	case InstrPop:
		return vm.executePop()
	case InstrAdd:
		return vm.executeAdd()
	case InstrSub:
		return vm.executeSub()
	case InstrMul:
		return vm.executeMul()
	case InstrDiv:
		return vm.executeDiv()
	case InstrMod:
		return vm.executeMod()
	case InstrEq:
		return vm.executeEq()
	case InstrNeq:
		return vm.executeNeq()
	case InstrLt:
		return vm.executeLt()
	case InstrGt:
		return vm.executeGt()
	case InstrLte:
		return vm.executeLte()
	case InstrGte:
		return vm.executeGte()
	case InstrLoad:
		return vm.executeLoad()
	case InstrStore:
		return vm.executeStore()
	case InstrJmp:
		return vm.executeJmp()
	case InstrJmpIfZero:
		return vm.executeJmpIfZero()
	case InstrJmpIfNeg:
		return vm.executeJmpIfNeg()
	case InstrJmpIfPos:
		return vm.executeJmpIfPos()
	case InstrCall:
		return vm.executeCall()
	case InstrRet:
		return vm.executeRet()
	case InstrHalt:
		return vm.executeHalt()
	default:
		return fmt.Errorf("unknown instruction: %d", instruction)
	}
}

func (vm *VM) executePush() error {
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	value := int(vm.bytecode[vm.currentState.PC])
	vm.currentState.Stack = append(vm.currentState.Stack, value)
	vm.currentState.PC++
	return nil
}

func (vm *VM) executePop() error {
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	varIdx := int(vm.bytecode[vm.currentState.PC])
	if varIdx >= len(vm.currentState.Locals) {
		return fmt.Errorf("variable index out of bounds: %d", varIdx)
	}
	vm.currentState.Locals[varIdx] = vm.currentState.Stack[len(vm.currentState.Stack)-1]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeAdd() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	vm.currentState.Stack = append(vm.currentState.Stack, a+b)
	return nil
}

func (vm *VM) executeSub() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	vm.currentState.Stack = append(vm.currentState.Stack, a-b)
	return nil
}

func (vm *VM) executeMul() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	vm.currentState.Stack = append(vm.currentState.Stack, a*b)
	return nil
}

func (vm *VM) executeDiv() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	vm.currentState.Stack = append(vm.currentState.Stack, a/b)
	return nil
}

func (vm *VM) executeMod() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	vm.currentState.Stack = append(vm.currentState.Stack, a%b)
	return nil
}

func (vm *VM) executeEq() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if a == b {
		vm.currentState.Stack = append(vm.currentState.Stack, 1)
	} else {
		vm.currentState.Stack = append(vm.currentState.Stack, 0)
	}
	return nil
}

func (vm *VM) executeNeq() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	vm.currentState.Stack = append(vm.currentState.Stack, 1)
	if a != b {
		return nil
	}
	vm.currentState.Stack = append(vm.currentState.Stack, 0)
	vm.currentState.PC += 2
	return nil
}

func (vm *VM) executeLt() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if b < a {
		vm.currentState.Stack = append(vm.currentState.Stack, 1)
	} else {
		vm.currentState.Stack = append(vm.currentState.Stack, 0)
	}
	return nil
}

func (vm *VM) executeGt() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if b > a {
		vm.currentState.Stack = append(vm.currentState.Stack, 1)
	} else {
		vm.currentState.Stack = append(vm.currentState.Stack, 0)
	}
	return nil
}

func (vm *VM) executeLte() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if b <= a {
		vm.currentState.Stack = append(vm.currentState.Stack, 1)
	} else {
		vm.currentState.Stack = append(vm.currentState.Stack, 0)
	}
	return nil
}

func (vm *VM) executeGte() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if b >= a {
		vm.currentState.Stack = append(vm.currentState.Stack, 1)
	} else {
		vm.currentState.Stack = append(vm.currentState.Stack, 0)
	}
	return nil
}

func (vm *VM) executeLoad() error {
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	varIdx := int(vm.bytecode[vm.currentState.PC])
	if varIdx >= len(vm.currentState.Locals) {
		return fmt.Errorf("variable index out of bounds: %d", varIdx)
	}
	vm.currentState.Stack = append(vm.currentState.Stack, vm.currentState.Locals[varIdx])
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeStore() error {
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	varIdx := int(vm.bytecode[vm.currentState.PC])
	if varIdx >= len(vm.currentState.Locals)-1 {
		vm.currentState.Locals = append(vm.currentState.Locals, 0)
	}
	vm.currentState.Locals[varIdx] = vm.currentState.Stack[len(vm.currentState.Stack)-1]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeJmp() error {
	// Read two bytes for jump address
	if vm.currentState.PC+1 >= len(vm.bytecode) {
		return fmt.Errorf("invalid jump address")
	}
	highByte := int(vm.bytecode[vm.currentState.PC])
	lowByte := int(vm.bytecode[vm.currentState.PC+1])
	jumpAddr := (highByte << 8) | lowByte
	vm.currentState.PC = jumpAddr
	return nil
}

func (vm *VM) executeJmpIfZero() error {
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	// Read two bytes for jump address
	if vm.currentState.PC+1 >= len(vm.bytecode) {
		return fmt.Errorf("invalid jump address")
	}
	highByte := int(vm.bytecode[vm.currentState.PC])
	lowByte := int(vm.bytecode[vm.currentState.PC+1])
	jumpAddr := (highByte << 8) | lowByte

	condition := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	// Pop the condition value
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]

	if condition == 0 {
		vm.currentState.PC = jumpAddr
	} else {
		vm.currentState.PC += 2 // Skip over jump address
	}
	return nil
}

func (vm *VM) executeJmpIfNeg() error {
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	// Read two bytes for jump address
	if vm.currentState.PC+1 >= len(vm.bytecode) {
		return fmt.Errorf("invalid jump address")
	}
	highByte := int(vm.bytecode[vm.currentState.PC])
	lowByte := int(vm.bytecode[vm.currentState.PC+1])
	jumpAddr := (highByte << 8) | lowByte

	condition := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	// Pop the condition value
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]

	if condition < 0 {
		vm.currentState.PC = jumpAddr
	} else {
		vm.currentState.PC += 2 // Skip over jump address
	}
	return nil
}

func (vm *VM) executeJmpIfPos() error {
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	// Read two bytes for jump address
	if vm.currentState.PC+1 >= len(vm.bytecode) {
		return fmt.Errorf("invalid jump address")
	}
	highByte := int(vm.bytecode[vm.currentState.PC])
	lowByte := int(vm.bytecode[vm.currentState.PC+1])
	jumpAddr := (highByte << 8) | lowByte

	condition := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	// Pop the condition value
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]

	if condition > 0 {
		vm.currentState.PC = jumpAddr
	} else {
		vm.currentState.PC += 2 // Skip over jump address
	}
	return nil
}

func (vm *VM) executeCall() error {
	funcIdx := int(vm.bytecode[vm.currentState.PC])

	if fn, ok := vm.functions[funcIdx]; ok {
		numArgs := int(vm.bytecode[vm.currentState.PC+1])
		args := make([]int, numArgs)
		for i := numArgs - 1; i >= 0; i-- {
			if len(vm.currentState.Stack) == 0 {
				return fmt.Errorf("stack underflow while getting function arguments")
			}
			args[i] = vm.currentState.Stack[len(vm.currentState.Stack)-1]
			vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]
		}

		result := fn(args)
		vm.currentState.Stack = append(vm.currentState.Stack, result)
		vm.currentState.PC += 2
		return nil
	}

	vm.currentState.CallStack = append(vm.currentState.CallStack, vm.currentState.PC)
	vm.currentState.PC = vm.currentState.Locals[vm.currentState.PC]
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeRet() error {
	if len(vm.currentState.CallStack) == 0 {
		return fmt.Errorf("call stack underflow")
	}
	vm.currentState.PC = vm.currentState.CallStack[len(vm.currentState.CallStack)-1]
	vm.currentState.CallStack = vm.currentState.CallStack[:len(vm.currentState.CallStack)-1]
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeHalt() error {
	vm.running = false
	return nil
}

func (vm *VM) RegisterFunction(idx int, fn GoFunction) {
	vm.functions[idx] = fn
}
