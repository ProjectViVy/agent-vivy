package commandpolicy

import "testing"

func TestExecutablePolicyNormalizesNamesAcrossPathStyles(t *testing.T) {
	for _, name := range []string{"bash", "/usr/bin/rm", `C:\\Windows\\System32\\cmd.exe`, "powershell.BAT"} {
		if !IsDeniedExecutable(name) {
			t.Errorf("IsDeniedExecutable(%q) = false", name)
		}
	}
	if IsDeniedExecutable("node") {
		t.Fatal("node must not be denied")
	}
	if got := NormalizeExecutableName(`C:\\Tools\\Node.EXE`); got != "node" {
		t.Fatalf("NormalizeExecutableName = %q, want node", got)
	}
}

func TestShellEscapePolicyIsNarrowerThanConfiguredDenylist(t *testing.T) {
	if !IsShellEscapeExecutable("cmd.exe") || !IsShellEscapeExecutable(".") {
		t.Fatal("host shell escapes must be shared")
	}
	if IsShellEscapeExecutable("rm") {
		t.Fatal("rm remains argument-sensitive in the shell classifier")
	}
}
