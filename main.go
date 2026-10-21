package main

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/DryHumour/grimroster/internal/character"
	"github.com/DryHumour/grimroster/internal/merge"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) < 2 || len(args) > 3 {
		return errors.New("usage: grimroster <grim-json-file> <gd-edit-html-file> [grim-game-directory]")
	}

	var (
		merged character.Character
		err    error
	)
	if len(args) == 3 {
		merged, err = merge.ReadFilesWithGameDir(args[0], args[1], args[2])
	} else {
		merged, err = merge.ReadFiles(args[0], args[1])
	}
	if err != nil {
		return err
	}
	if err := json.MarshalWrite(output, merged); err != nil {
		return fmt.Errorf("encode merged character JSON: %w", err)
	}
	if _, err := io.WriteString(output, "\n"); err != nil {
		return fmt.Errorf("write merged character JSON: %w", err)
	}
	for _, diagnostic := range merged.Diagnostics {
		if diagnostic.Severity == character.SeverityError {
			return errors.New("merged character contains error-severity integrity diagnostics")
		}
	}
	return nil
}
