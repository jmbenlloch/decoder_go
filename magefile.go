//go:build mage
// +build mage

package main

import (
	"fmt"
	"os/exec"

	"github.com/magefile/mage/mg"
)

// Default target to run when none is specified
// If not set, running mage will list available targets
var Default = Build

// A build step that requires additional params, or platform specific steps for example
func Build() error {
	mg.Deps(BuildDecoder)
	fmt.Println("Compilation finished")
	return nil
}

func BuildDecoder() error {
	fmt.Println("Building decoder executable...")
	cmd := exec.Command("go", "build", "-o", "./decoder", "./decoder")
	return cmd.Run()
}
