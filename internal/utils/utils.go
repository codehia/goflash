package utils

import (
	"fmt"
	"os"
)

func StringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func GetEnv(key string, fallback ...string) (string, error) {
	if value, ok := os.LookupEnv(key); ok {
		return value, nil
	}
	if len(fallback) > 0 {
		return fallback[0], nil
	}
	return "", fmt.Errorf("env var %q is not set", key)
}
