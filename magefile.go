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

// Test runs the unit and fixture-based tests (no DB or big-data access needed).
// Must run in an environment with CGO + libhdf5 (the nextmgmt/next-decoder
// container); use ./test.sh from the host.
func Test() error {
	return runTests(nil)
}

// TestDB additionally enables the live-DB integration test, against a
// disposable local container by default, or against an overridden target via
// the DECODER_TEST_DB_{HOST,USER,PASS,NAME} env vars (see
// pkg/database_test.go and pkg/testdata/hddemo_616_seed.sql).
func TestDB() error {
	return runTests([]string{"DECODER_TEST_DB=1"})
}

func runTests(extraEnv []string) error {
	fmt.Println("Running tests...")
	cmd := exec.Command("go", "test", "./pkg/...", "./decoder/...")
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		fmt.Sprintf("CGO_LDFLAGS=%s", os.Getenv("CGO_LDFLAGS")),
		fmt.Sprintf("CGO_CFLAGS=%s", os.Getenv("CGO_CFLAGS")))
	cmd.Env = append(cmd.Env, extraEnv...)
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
