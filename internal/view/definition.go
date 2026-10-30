package view

import (
	"fmt"
	"strings"
)

// Definition is the parsed content of a view's _view file.
type Definition struct {
	Filter string
	Modify string
}

// Writable reports whether the definition carries modification arguments.
func (d Definition) Writable() bool { return d.Modify != "" }

// SplitDefinition separates the filter line from the optional modification line.
func SplitDefinition(raw string) (Definition, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}

	def := Definition{Filter: strings.TrimSpace(lines[0])}
	if len(lines) == 1 {
		return def, nil
	}
	def.Modify = strings.TrimSpace(lines[1])
	for _, line := range lines[2:] {
		if strings.TrimSpace(line) != "" {
			return Definition{}, fmt.Errorf("view definition has more than two non-empty lines")
		}
	}
	return def, nil
}

// SplitArgs splits a rendered modification line into shell-style arguments
// without performing variable expansion, globbing, or command substitution.
func SplitArgs(line string) ([]string, error) {
	var args []string
	var token strings.Builder
	started := false
	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if !started {
			return
		}
		args = append(args, token.String())
		token.Reset()
		started = false
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			token.WriteByte(c)
			started = true
			escaped = false
			continue
		}
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				token.WriteByte(c)
			}
			started = true
		case inDouble:
			switch c {
			case '"':
				inDouble = false
			case '\\':
				escaped = true
			default:
				token.WriteByte(c)
			}
			started = true
		default:
			switch c {
			case ' ', '\t', '\n', '\r':
				flush()
			case '\'':
				inSingle = true
				started = true
			case '"':
				inDouble = true
				started = true
			case '\\':
				escaped = true
				started = true
			default:
				token.WriteByte(c)
				started = true
			}
		}
	}

	switch {
	case escaped:
		return nil, fmt.Errorf("trailing escape in modification line")
	case inSingle:
		return nil, fmt.Errorf("unterminated single quote in modification line")
	case inDouble:
		return nil, fmt.Errorf("unterminated double quote in modification line")
	}
	flush()
	return args, nil
}
