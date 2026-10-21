package backend

import (
	"encoding/json"
	"fmt"
)

type Task struct {
	UUID        string   `json:"uuid"`
	Description string   `json:"description"`
	Project     string   `json:"project"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
}

func ParseExport(data []byte) ([]Task, error) {
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parsing task export: %w", err)
	}
	return tasks, nil
}
