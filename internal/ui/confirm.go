package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type ConfirmOptions struct {
	Action  string
	Force   bool
	Confirm string
	NoInput bool
	UseTTY  bool
	Out     *os.File
	In      *os.File
}

func RequireConfirmation(opts ConfirmOptions) error {
	if opts.Force {
		return nil
	}
	if opts.Confirm != "" {
		if opts.Confirm == opts.Action {
			return nil
		}
		return fmt.Errorf("--confirm must equal %q", opts.Action)
	}
	if opts.NoInput || !opts.UseTTY {
		return fmt.Errorf("confirmation required: pass --force or --confirm=%s", opts.Action)
	}
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	if _, err := fmt.Fprintf(out, "Type %q to confirm: ", opts.Action); err != nil {
		return err
	}
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(line)
	if line != opts.Action {
		return fmt.Errorf("confirmation did not match")
	}
	return nil
}
