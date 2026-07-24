// Command toonjson converts one TOON document on stdin to JSON on stdout.
// It is used by live agent contract tests to inspect the exact compact payload
// sent to models without changing the production TOON output format.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	toon "github.com/toon-format/toon-go"
)

func main() {
	if err := convert(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "convert TOON to JSON: %v\n", err)
		os.Exit(1)
	}
}

func convert(input io.Reader, output io.Writer) error {
	body, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	value, err := toon.Decode(body)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
