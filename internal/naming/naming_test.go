package naming

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFileNameBasic(t *testing.T) {
	got := FileName("Qt6 signals and slots", "f88d7a5f-8d68-44fd-8cd3-3725592b673e")
	want := "Qt6 signals and slots.f88d7a5f.txt"
	if got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

func TestFileNameSlashDescription(t *testing.T) {
	got := FileName("Fix A/B testing", "a1b2c3d4-0000-0000-0000-000000000000")
	want := "Fix A_B testing.a1b2c3d4.txt"
	if got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

func TestFileNameIllegalChars(t *testing.T) {
	got := FileName(`a\b:c*d?e"f<g>h|i`, "a1b2c3d4-0000-0000-0000-000000000000")
	want := "a_b_c_d_e_f_g_h_i.a1b2c3d4.txt"
	if got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

func TestFileNameReservedDevice(t *testing.T) {
	got := FileName("CON", "a1b2c3d4-0000-0000-0000-000000000000")
	if !strings.HasPrefix(got, "_CON.") {
		t.Errorf("reserved device not mangled: %q", got)
	}
}

func TestFileNameTrailingDotsAndSpaces(t *testing.T) {
	got := FileName("Do taxes.  ", "a1b2c3d4-0000-0000-0000-000000000000")
	want := "Do taxes.a1b2c3d4.txt"
	if got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

func TestFileNameTruncation(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := FileName(long, "a1b2c3d4-0000-0000-0000-000000000000")
	if len(got) > 255 {
		t.Errorf("name length %d exceeds 255", len(got))
	}
	if !strings.HasSuffix(got, ".a1b2c3d4.txt") {
		t.Errorf("suffix/extension lost: %q", got)
	}
}

func TestFileNameTruncationMultibyte(t *testing.T) {
	long := strings.Repeat("é", 200)
	got := FileName(long, "a1b2c3d4-0000-0000-0000-000000000000")
	if len(got) > 255 || !utf8.ValidString(strings.TrimSuffix(strings.TrimSuffix(got, ".txt"), ".a1b2c3d4")) {
		t.Errorf("invalid truncation: %q (%d bytes)", got, len(got))
	}
	if !strings.HasSuffix(got, ".a1b2c3d4.txt") {
		t.Errorf("suffix/extension lost: %q", got)
	}
}

func TestFileNameDuplicateDescriptions(t *testing.T) {
	a := FileName("Do taxes", "a1b2c3d4-1111-1111-1111-111111111111")
	b := FileName("Do taxes", "e5f6a7b8-2222-2222-2222-222222222222")
	if a == b {
		t.Error("distinct UUIDs must yield distinct names")
	}
	if !strings.HasSuffix(a, ".a1b2c3d4.txt") || !strings.HasSuffix(b, ".e5f6a7b8.txt") {
		t.Errorf("suffixes wrong: %q %q", a, b)
	}
}

func TestFileNameSuffixStability(t *testing.T) {
	before := FileName("Do taxes", "a1b2c3d4-1111-1111-1111-111111111111")
	after := FileName("Do taxes", "a1b2c3d4-1111-1111-1111-111111111111")
	if before != after {
		t.Error("name must be stable across listings")
	}
}

func TestIsMintedName(t *testing.T) {
	yes := []string{
		"Do taxes.a1b2c3d4.txt",
		"Qt6 signals and slots.f88d7a5f.txt",
		".12345678.txt",
	}
	no := []string{
		"Do taxes.txt",
		"Do taxes.a1b2c3d.txt",
		"Do taxes.a1b2c3d4e.txt",
		"Do taxes.a1b2c3g4.txt",
		"Do taxes.A1B2C3D4.txt",
		"Do taxes.a1b2c3d4.md",
		"Pick up Milk",
	}
	for _, n := range yes {
		if !IsMintedName(n) {
			t.Errorf("IsMintedName(%q) = false, want true", n)
		}
	}
	for _, n := range no {
		if IsMintedName(n) {
			t.Errorf("IsMintedName(%q) = true, want false", n)
		}
	}
}
