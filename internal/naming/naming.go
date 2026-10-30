package naming

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxNameLen    = 255
	suffixFmt     = "%s.%s.txt"
	SuffixLenBase = 13 // "." + 8 hex + ".txt"
)

var illegalChars = map[rune]bool{
	'\\': true, '/': true, ':': true, '*': true, '?': true,
	'"': true, '<': true, '>': true, '|': true,
}

var reservedDevices = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ShortUUID returns the leading 8 hexadecimal characters of a full UUID.
func ShortUUID(uuid string) string {
	if len(uuid) < 8 {
		return ""
	}
	return uuid[:8]
}

// FileName derives the deterministic task file name for a description
// and full UUID: sanitized description, short-UUID suffix, .txt
// extension, truncated to 255 bytes with suffix and extension retained.
func FileName(description, uuid string) string {
	short := ShortUUID(uuid)
	base := sanitize(description)
	room := maxNameLen - len(short) - SuffixLenBase
	base = truncateBytes(base, room)
	if base == "" {
		base = "task"
	}
	return fmt.Sprintf(suffixFmt, base, short)
}

func sanitize(description string) string {
	var b strings.Builder
	for _, r := range description {
		if illegalChars[r] || r < 0x20 || r == 0x7f {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	base := b.String()
	first := base
	if i := strings.IndexByte(base, '.'); i >= 0 {
		first = base[:i]
	}
	if reservedDevices[strings.ToLower(first)] {
		base = "_" + base
	}
	base = strings.TrimRight(base, ". ")
	return base
}

func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimRight(cut, ". ")
}

const hexDigits = "0123456789abcdef"

// IsMintedName reports whether a file name matches the pattern the
// filesystem mints for task files: <anything>.<8-hex>.txt.
func IsMintedName(name string) bool {
	if !strings.HasSuffix(name, ".txt") {
		return false
	}
	body := strings.TrimSuffix(name, ".txt")
	dot := strings.LastIndexByte(body, '.')
	if dot < 0 || len(body)-dot-1 != 8 {
		return false
	}
	for _, c := range body[dot+1:] {
		if !strings.ContainsRune(hexDigits, c) {
			return false
		}
	}
	return true
}
