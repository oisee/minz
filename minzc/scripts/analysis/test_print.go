package main

import (
    "fmt"
    "os"
)

func main() {
    fmt.Println("Test print to stdout")
    fmt.Fprintf(os.Stderr, "Test print to stderr\n")
}