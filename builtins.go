package main

import "fmt"

// RegisterBuiltins registers all built-in functions with the VM
func RegisterBuiltins(vm *VM) {
	// Register the print function
	vm.RegisterFunction(builtinFunctions["print"], printFunc(vm))
}

// printFunc creates a print function closure that has access to the VM
func printFunc(vm *VM) GoFunction {
	return func(args []Value) Value {
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
	}
}
