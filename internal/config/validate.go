package config

import (
	"fmt"
	"strings"
)

func CanonicalizePRState(raw string) (string, error) {
	state := strings.ToLower(strings.TrimSpace(raw))
	if state == "" || state == "all" {
		return "all", nil
	}
	switch state {
	case "open", "closed", "merged":
		return state, nil
	default:
		return "", fmt.Errorf("invalid pr_state: %s", raw)
	}
}

func CanonicalizePRPrefer(raw string) (string, error) {
	prefer := strings.ToLower(strings.TrimSpace(raw))
	if prefer == "" {
		return "open", nil
	}
	switch prefer {
	case "open", "merged", "closed", "none":
		return prefer, nil
	default:
		return "", fmt.Errorf("invalid pr_prefer: %s", raw)
	}
}

func ValidatePRLimit(limit int) error {
	if limit < 1 || limit > 20 {
		return fmt.Errorf("pr_limit must be between 1 and 20")
	}
	return nil
}

func NormalizeUI(values UISettings) (UISettings, error) {
	var err error
	values.Fields = strings.TrimSpace(values.Fields)
	values.Sort = strings.TrimSpace(values.Sort)

	values.PRState, err = CanonicalizePRState(values.PRState)
	if err != nil {
		return values, err
	}
	values.PRPrefer, err = CanonicalizePRPrefer(values.PRPrefer)
	if err != nil {
		return values, err
	}
	if err := ValidatePRLimit(values.PRLimit); err != nil {
		return values, err
	}
	return values, nil
}
