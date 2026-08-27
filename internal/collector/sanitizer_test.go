package collector

import (
	"testing"
)

func TestSanitizeProcessName(t *testing.T) {
	valid := []string{
		"appserver.exe",
		"dbaccess.exe",
		"services.exe",
		"totvs-service_01.exe",
	}

	for _, v := range valid {
		res, err := SanitizeProcessName(v)
		if err != nil {
			t.Errorf("expected valid process name %s to pass, got err: %v", v, err)
		}
		if res != v {
			t.Errorf("expected %s, got %s", v, res)
		}
	}

	invalid := []string{
		"appserver.exe; rm -rf /",
		"dbaccess.exe | whoami",
		"cmd.exe",
		"powershell.exe",
		"app`whoami`.exe",
		"app$(calc).exe",
		"test&dir",
		"",
	}

	for _, inv := range invalid {
		_, err := SanitizeProcessName(inv)
		if err == nil {
			t.Errorf("expected malicious or invalid process name '%s' to be rejected", inv)
		}
	}
}

func TestSanitizePath(t *testing.T) {
	valid := []struct {
		input    string
		expected string
	}{
		{"C$\\totvs\\protheus\\bin\\appserver\\console.log", "C$\\totvs\\protheus\\bin\\appserver\\console.log"},
		{"totvs/dbaccess/dbaccess.log", "totvs\\dbaccess\\dbaccess.log"},
		{"C:\\protheus\\data", "C:\\protheus\\data"},
	}

	for _, v := range valid {
		res, err := SanitizePath(v.input)
		if err != nil {
			t.Errorf("expected path %s to pass, got err: %v", v.input, err)
		}
		if res != v.expected {
			t.Errorf("expected %s, got %s", v.expected, res)
		}
	}

	invalid := []string{
		"C$\\logs; calc.exe",
		"C$\\logs | net user",
		"C$\\logs`id`",
		"C$\\logs$(whoami)",
		"C$\\logs\nrmdir /s /q C:\\",
		"C$\\logs\" && echo 1",
		"",
	}

	for _, inv := range invalid {
		_, err := SanitizePath(inv)
		if err == nil {
			t.Errorf("expected malicious path '%s' to be rejected", inv)
		}
	}
}

func TestEscapePowerShellString(t *testing.T) {
	input := "user' OR '1'='1"
	expected := "user'' OR ''1''=''1"
	got := EscapePowerShellString(input)
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
