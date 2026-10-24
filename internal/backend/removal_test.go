package backend

import (
	"strings"
	"testing"
)

func TestParseRemoveAction(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    RemoveAction
		wantErr bool
	}{
		{"complete", "complete", RemoveComplete, false},
		{"old done value", "done", "", true},
		{"delete", "delete", RemoveDelete, false},
		{"empty", "", "", true},
		{"unknown", "archive", "", true},
		{"wrong case", "Done", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRemoveAction(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRemoveAction(%q) = %q, want error", tt.value, got)
				}
				if tt.value != "" && !strings.Contains(err.Error(), tt.value) {
					t.Errorf("error %q does not name input %q", err, tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRemoveAction(%q): %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("ParseRemoveAction(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
