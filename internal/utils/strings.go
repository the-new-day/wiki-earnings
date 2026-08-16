package utils

import "strings"

func ContainsAny(target string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(target, sub) {
			return true
		}
	}
	return false
}
