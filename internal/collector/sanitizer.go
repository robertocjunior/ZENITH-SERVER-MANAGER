package collector

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// safeProcessRegex only allows alphanumeric characters, dot, underscore, and hyphen.
	safeProcessRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

	// safePathRegex allows standard Windows file paths, administrative shares (C$), UNC paths, and drive letters.
	// Rejects shell metacharacters: ;, &, |, `, (, ), {, }, <, >, ", ', newline, tab.
	safePathRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.\\/:$]+$`)
)

var (
	ErrInvalidProcessName = errors.New("invalid process name: contains disallowed characters")
	ErrInvalidPath        = errors.New("invalid file or share path: contains disallowed characters")
	ErrCommandInjection   = errors.New("security validation failed: potential command injection detected")
)

// SanitizeProcessName validates and escapes process names to be passed safely to WMI / PowerShell.
func SanitizeProcessName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("process name cannot be empty")
	}

	// Length constraint
	if len(trimmed) > 100 {
		return "", errors.New("process name exceeds maximum length (100 characters)")
	}

	if !safeProcessRegex.MatchString(trimmed) {
		return "", fmt.Errorf("%w: '%s'", ErrInvalidProcessName, trimmed)
	}

	// Ensure no dangerous keywords or command injection patterns
	lower := strings.ToLower(trimmed)
	dangerous := []string{"invoke-", "iex", "cmd.exe", "powershell.exe", "start-process", "rmdir", "del", "format"}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return "", fmt.Errorf("%w: '%s'", ErrCommandInjection, trimmed)
		}
	}

	return trimmed, nil
}

// SanitizePath validates file paths or share names used in SMB or WinRM.
func SanitizePath(p string) (string, error) {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return "", errors.New("path cannot be empty")
	}

	if len(trimmed) > 260 {
		return "", errors.New("path exceeds maximum length (260 characters)")
	}

	// Reject dangerous command injection sequences: ;, &, |, `, $(, ${, <, >, ", ', \n, \r
	forbidden := []string{";", "&", "|", "`", "$(", "${", "<", ">", "\"", "'", "\n", "\r", "{", "}", "%"}
	for _, char := range forbidden {
		if strings.Contains(trimmed, char) {
			return "", fmt.Errorf("%w: contains forbidden sequence '%s'", ErrCommandInjection, char)
		}
	}

	if !safePathRegex.MatchString(trimmed) {
		return "", fmt.Errorf("%w: '%s'", ErrInvalidPath, trimmed)
	}

	// Normalize forward slashes to Windows backslashes for consistency
	normalized := strings.ReplaceAll(trimmed, "/", "\\")
	return normalized, nil
}

// EscapePowerShellString escapes a string for safe inclusion in single-quoted PowerShell strings.
func EscapePowerShellString(s string) string {
	// In PowerShell, single quotes are escaped by doubling them (' -> '')
	return strings.ReplaceAll(s, "'", "''")
}
