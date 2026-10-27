package view

import (
	"context"
	"fmt"
	"strings"
)

// Template is a custom view filter with zero or more command blocks.
type Template struct {
	parts []part
}

type part struct {
	literal string
	command string
}

// Runner executes one command block and returns its standard output.
type Runner interface {
	Run(ctx context.Context, command string) ([]byte, error)
}

// Parse splits a filter into literal text and {{ command }} blocks.
func Parse(raw string) (*Template, error) {
	t := &Template{}
	var literal strings.Builder

	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		t.parts = append(t.parts, part{literal: literal.String()})
		literal.Reset()
	}

	for i := 0; i < len(raw); {
		switch {
		case strings.HasPrefix(raw[i:], `\{{`):
			literal.WriteString("{{")
			i += 3
		case strings.HasPrefix(raw[i:], "{{"):
			flushLiteral()
			end := strings.Index(raw[i+2:], "}}")
			if end < 0 {
				return nil, fmt.Errorf("unclosed command block")
			}
			end += i + 2
			command := strings.TrimSpace(raw[i+2 : end])
			if command == "" {
				return nil, fmt.Errorf("empty command block")
			}
			t.parts = append(t.parts, part{command: command})
			i = end + 2
		case raw[i] == '\\' && i == len(raw)-1:
			return nil, fmt.Errorf("trailing escape")
		default:
			literal.WriteByte(raw[i])
			i++
		}
	}
	flushLiteral()

	return t, nil
}

// Render replaces each command block with the validated command output.
func (t *Template) Render(ctx context.Context, runner Runner) (string, error) {
	var out strings.Builder
	for _, p := range t.parts {
		if p.command == "" {
			out.WriteString(p.literal)
			continue
		}
		if runner == nil {
			return "", fmt.Errorf("command %q: no command runner", p.command)
		}
		raw, err := runner.Run(ctx, p.command)
		if err != nil {
			return "", err
		}
		value, err := validateOutput(raw)
		if err != nil {
			return "", fmt.Errorf("command %q: %w", p.command, err)
		}
		out.WriteString(value)
	}
	return out.String(), nil
}

func validateOutput(raw []byte) (string, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("empty output")
	}
	if strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("output contains NUL")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("output contains an embedded newline")
	}
	return value, nil
}
