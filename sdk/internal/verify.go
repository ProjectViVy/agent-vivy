package sdk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"agent-vivy/sdk/plugin"
)

// Report is the result of verifying one plugin directory.
type Report struct {
	Dir    string
	OK     bool
	Issues []string
}

// Verify checks one plugins/<name> source package. It never writes an
// executable.
func Verify(dir string) (Report, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Report{}, fmt.Errorf("sdk: abs %s: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Report{}, fmt.Errorf("sdk: plugin dir: %w", err)
	}
	if !info.IsDir() {
		return Report{}, fmt.Errorf("sdk: %s is not a directory", abs)
	}
	rep := Report{Dir: abs}
	man, err := loadManifest(abs)
	if err != nil {
		rep.Issues = append(rep.Issues, err.Error())
		return rep, nil
	}
	rep.Issues = append(rep.Issues, checkManifest(abs, man)...)
	files, parseIssues := parsePluginSources(abs)
	rep.Issues = append(rep.Issues, parseIssues...)
	rep.Issues = append(rep.Issues, checkSources(files, plugin.Seam(man.Seam))...)
	if len(rep.Issues) == 0 {
		rep.Issues = append(rep.Issues, checkLinkable(abs)...)
	}
	rep.OK = len(rep.Issues) == 0
	return rep, nil
}

func checkLinkable(dir string) []string {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}
	if _, err := os.Stat(goBin); err != nil {
		return []string{"go toolchain missing; cannot prove the package is linkable"}
	}
	cmd := exec.Command(goBin, "list", ".")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return []string{fmt.Sprintf("package is not linkable: %s", strings.TrimSpace(string(out)))}
	}
	if strings.TrimSpace(string(out)) == "" {
		return []string{"go list returned no package path"}
	}
	return nil
}
