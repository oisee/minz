package main

import (
    "fmt"
    "os"
    "os/exec"
)

func main() {
    // Run compiler with debug environment variable
    cmd := exec.Command("./minzc/minzc", "examples/lambda_simple_fix.minz", "-o", "/dev/null", "--debug")
    cmd.Env = append(os.Environ(), "DEBUG_LAMBDA=1")
    output, err := cmd.CombinedOutput()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
    fmt.Println(string(output))
}