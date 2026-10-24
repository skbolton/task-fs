package backend

import "fmt"

// RemoveAction is what a removal gesture does to the matching tasks.
type RemoveAction string

const (
	RemoveComplete RemoveAction = "complete"
	RemoveDelete   RemoveAction = "delete"
)

func ParseRemoveAction(value string) (RemoveAction, error) {
	switch action := RemoveAction(value); action {
	case RemoveComplete, RemoveDelete:
		return action, nil
	default:
		return "", fmt.Errorf("invalid removal action %q: must be %q or %q", value, RemoveComplete, RemoveDelete)
	}
}
