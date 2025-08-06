package main

import (
    "fmt"
    "./minzc/pkg/semantic"
    "./minzc/pkg/ir"
)

func main() {
    a := semantic.NewAnalyzer()
    scope := a.GetCurrentScope()
    
    // Check if String is defined
    sym := scope.Lookup("String")
    if sym \!= nil {
        fmt.Printf("String found: %T\n", sym)
        if ts, ok := sym.(*semantic.TypeSymbol); ok {
            fmt.Printf("  Type: %T\n", ts.Type)
        }
    } else {
        fmt.Println("String not found")
    }
}
