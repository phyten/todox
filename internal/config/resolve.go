package config

import "strings"

func ResolveString(def string, values ...*string) string {
	result := def
	for _, v := range values {
		if v != nil {
			result = *v
		}
	}
	return result
}

func ResolveInt(def int, values ...*int) int {
	result := def
	for _, v := range values {
		if v != nil {
			result = *v
		}
	}
	return result
}

func ResolveBool(def bool, values ...*bool) bool {
	result := def
	for _, v := range values {
		if v != nil {
			result = *v
		}
	}
	return result
}

func ResolveStrings(def []string, values ...*[]string) []string {
	result := cloneStrings(def)
	for _, v := range values {
		if v != nil {
			if len(*v) == 0 {
				result = []string{}
				continue
			}
			result = cloneStrings(*v)
		}
	}
	return result
}

func ResolveAndTrim(def string, values ...*string) string {
	value := ResolveString(def, values...)
	return strings.TrimSpace(value)
}
