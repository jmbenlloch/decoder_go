//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg"
)

// Default target to run when none is specified
// If not set, running mage will list available targets
var Default = Build

// A build step that requires additional params, or platform specific steps for example
func Build() error {
	mg.Deps(BuildDecoder)
	mg.Deps(BuildMeasureAlgos)
	fmt.Println("Compilation finished")
	return nil
}

func BuildDecoder() error {
	fmt.Println("Building decoder executable...")
	ldflags := os.Getenv("CGO_LDFLAGS")
	cflags := os.Getenv("CGO_CFLAGS")
	cmd := exec.Command("go", "build", "-o", "./bin/decoder", "./decoder")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CGO_ENABLED=1"),
		fmt.Sprintf("CGO_LDFLAGS=%s", ldflags),
		fmt.Sprintf("CGO_CFLAGS=%s", cflags))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func BuildMeasureAlgos() error {
	fmt.Println("Building measureAlgos executable...")
	ldflags := os.Getenv("CGO_LDFLAGS")
	cflags := os.Getenv("CGO_CFLAGS")
	cmd := exec.Command("go", "build", "-o", "./bin/measureAlgos", "./measureAlgos")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CGO_ENABLED=1"),
		fmt.Sprintf("CGO_LDFLAGS=%s", ldflags),
		fmt.Sprintf("CGO_CFLAGS=%s", cflags))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
