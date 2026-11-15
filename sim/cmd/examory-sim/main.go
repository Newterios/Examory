// Command examory-sim generates paired behaviour-level quiz event logs.
//
//	examory-sim generate [flags]
//
// See sim/README.md for the model and the meaning of every output.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Newterios/Examory/sim/internal/events"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `usage: examory-sim generate [flags]
       examory-sim validate FILE.csv...
       examory-sim version

Run "examory-sim generate -h" for the flags.
`

// run executes the CLI and returns the process exit code: 0 on success,
// 2 on invalid input or any other error.
func run(args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "examory-sim: internal error: %v\n", r)
			code = 2
		}
	}()
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "generate":
		if err := generate(args[1:], stdout, stderr); err != nil {
			if errors.Is(err, errHelp) {
				return 0
			}
			fmt.Fprintf(stderr, "examory-sim: %v\n", err)
			return 2
		}
		return 0
	case "validate":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		for _, path := range args[1:] {
			if err := validateFile(path, stdout); err != nil {
				fmt.Fprintf(stderr, "examory-sim: %s: %v\n", path, err)
				return 2
			}
		}
		return 0
	case "version", "--version", "-version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "examory-sim: unknown command %q\n%s", args[0], usage)
		return 2
	}
}

// validateFile checks an event CSV with the Go re-implementation of the
// read_attempts invariants (without answer keys, so score is range-checked
// only).
func validateFile(path string, stdout io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sum, err := events.Validate(f, nil)
	if err != nil {
		return err
	}
	b, err := json.Marshal(map[string]any{"file": path, "events": sum.Events, "attempts": sum.Attempts,
		"attempts_by_type": sum.ByType, "answers_changed": sum.Changes})
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(b))
	return nil
}
