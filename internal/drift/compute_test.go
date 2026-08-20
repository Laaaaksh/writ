package drift

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Laaaaksh/writ/internal/writ"
)

// newTestRepo creates a git repo with an initial commit on "main". Tests call
// markBase once their pre-existing fixture state is committed, then make the
// changes under test on top of that.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test")
	writeFile(t, dir, "README.md", "seed\n")
	git(t, dir, "add", "README.md")
	git(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, repoDir, rel, content string) {
	t.Helper()
	full := filepath.Join(repoDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// markBase creates the "base" branch at the current HEAD.
func markBase(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "branch", "base")
}

func findChange(fcs []FileChange, path string) (FileChange, bool) {
	for _, fc := range fcs {
		if fc.Path == path {
			return fc, true
		}
	}
	return FileChange{}, false
}

func testWrit(scope ...string) *writ.Writ {
	return &writ.Writ{Base: "base", Scope: scope}
}

func TestCompute_ModificationInScope(t *testing.T) {
	dir := newTestRepo(t)
	markBase(t, dir)
	writeFile(t, dir, "app/foo.go", "package app\n")
	git(t, dir, "add", "app/foo.go")
	git(t, dir, "commit", "-q", "-m", "add foo")
	writeFile(t, dir, "app/foo.go", "package app\n\nfunc Foo() {}\n")
	git(t, dir, "add", "app/foo.go")
	git(t, dir, "commit", "-q", "-m", "extend foo")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(report.Drift) != 0 {
		t.Errorf("expected no drift, got %+v", report.Drift)
	}
	fc, ok := findChange(report.InScope, "app/foo.go")
	if !ok {
		t.Fatalf("expected app/foo.go in scope, got %+v", report.InScope)
	}
	if fc.Added == 0 {
		t.Errorf("expected added > 0, got %+v", fc)
	}
}

func TestCompute_ModificationOutOfScope(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "config/settings.rb", "A = 1\n")
	git(t, dir, "add", "config/settings.rb")
	git(t, dir, "commit", "-q", "-m", "add settings")
	markBase(t, dir)
	writeFile(t, dir, "config/settings.rb", "A = 1\nB = 2\n")
	// leave uncommitted (unstaged) to also exercise the working-tree diff path

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(report.InScope) != 0 {
		t.Errorf("expected no in-scope changes, got %+v", report.InScope)
	}
	fc, ok := findChange(report.Drift, "config/settings.rb")
	if !ok {
		t.Fatalf("expected config/settings.rb in drift, got %+v", report.Drift)
	}
	if fc.Added != 1 {
		t.Errorf("expected 1 added line, got %+v", fc)
	}
}

func TestCompute_UntrackedFileOutOfScope(t *testing.T) {
	dir := newTestRepo(t)
	markBase(t, dir)
	writeFile(t, dir, "scratch notes.txt", "line one\nline two\n")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	fc, ok := findChange(report.Drift, "scratch notes.txt")
	if !ok {
		t.Fatalf("expected untracked file in drift, got %+v", report.Drift)
	}
	if fc.Added != 2 {
		t.Errorf("expected 2 added lines, got %+v", fc)
	}
	if len(report.InScope) != 0 {
		t.Errorf("expected no in-scope changes, got %+v", report.InScope)
	}
}

func TestCompute_Deletion(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "app/gone.go", "package app\nfunc Gone() {}\n")
	git(t, dir, "add", "app/gone.go")
	git(t, dir, "commit", "-q", "-m", "add gone")
	markBase(t, dir)
	git(t, dir, "rm", "-q", "app/gone.go")
	git(t, dir, "commit", "-q", "-m", "remove gone")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	fc, ok := findChange(report.InScope, "app/gone.go")
	if !ok {
		t.Fatalf("expected app/gone.go in scope, got %+v / %+v", report.InScope, report.Drift)
	}
	if fc.Deleted == 0 {
		t.Errorf("expected deleted > 0, got %+v", fc)
	}
	if fc.Added != 0 {
		t.Errorf("expected added == 0 for a pure deletion, got %+v", fc)
	}
}

func TestCompute_RenameIntoScope(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "misc/mover.go", "package misc\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\n")
	git(t, dir, "add", "misc/mover.go")
	git(t, dir, "commit", "-q", "-m", "add mover")
	markBase(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "mv", "misc/mover.go", "app/mover.go")
	git(t, dir, "commit", "-q", "-m", "move mover into app")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if _, ok := findChange(report.Drift, "misc/mover.go"); ok {
		t.Errorf("old path should not appear in drift: %+v", report.Drift)
	}
	if _, ok := findChange(report.InScope, "app/mover.go"); !ok {
		t.Fatalf("expected app/mover.go (new path) in scope, got %+v / %+v", report.InScope, report.Drift)
	}
}

func TestCompute_RenameOutOfScope(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "app/mover.go", "package app\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\n")
	git(t, dir, "add", "app/mover.go")
	git(t, dir, "commit", "-q", "-m", "add mover")
	markBase(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "misc"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "mv", "app/mover.go", "misc/mover.go")
	git(t, dir, "commit", "-q", "-m", "move mover out of app")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if _, ok := findChange(report.InScope, "app/mover.go"); ok {
		t.Errorf("old path should not appear in scope: %+v", report.InScope)
	}
	if _, ok := findChange(report.Drift, "misc/mover.go"); !ok {
		t.Fatalf("expected misc/mover.go (new path) in drift, got %+v / %+v", report.InScope, report.Drift)
	}
}

func TestCompute_BinaryFile(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "app/blob.bin", "seed\x00binary\x01data")
	git(t, dir, "add", "app/blob.bin")
	git(t, dir, "commit", "-q", "-m", "add binary")
	markBase(t, dir)
	writeFile(t, dir, "app/blob.bin", "changed\x00binary\x01data\x02more")
	git(t, dir, "add", "app/blob.bin")
	git(t, dir, "commit", "-q", "-m", "change binary")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	fc, ok := findChange(report.InScope, "app/blob.bin")
	if !ok {
		t.Fatalf("expected app/blob.bin in scope, got %+v / %+v", report.InScope, report.Drift)
	}
	if fc.Added != 0 || fc.Deleted != 0 {
		t.Errorf("expected 0/0 for a binary file, got %+v", fc)
	}
}

func TestCompute_WritStateFileExcludedFromBothScopeAndDrift(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, writ.Dir+"/current.toml", "id = \"seed\"\n")
	git(t, dir, "add", writ.Dir)
	git(t, dir, "commit", "-q", "-m", "add writ state")
	markBase(t, dir)
	writeFile(t, dir, writ.Dir+"/current.toml", "id = \"seed\"\nintent = \"more\"\n")
	git(t, dir, "add", writ.Dir)
	git(t, dir, "commit", "-q", "-m", "advance writ state")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if _, ok := findChange(report.InScope, writ.Dir+"/current.toml"); ok {
		t.Errorf("expected %s/current.toml not in scope, got %+v", writ.Dir, report.InScope)
	}
	if _, ok := findChange(report.Drift, writ.Dir+"/current.toml"); ok {
		t.Errorf("expected %s/current.toml not in drift, got %+v", writ.Dir, report.Drift)
	}
}

func TestCompute_UntrackedWritStateFileExcludedFromBothScopeAndDrift(t *testing.T) {
	dir := newTestRepo(t)
	markBase(t, dir)
	writeFile(t, dir, writ.Dir+"/current.toml", "id = \"seed\"\n")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if _, ok := findChange(report.InScope, writ.Dir+"/current.toml"); ok {
		t.Errorf("expected untracked %s/current.toml not in scope, got %+v", writ.Dir, report.InScope)
	}
	if _, ok := findChange(report.Drift, writ.Dir+"/current.toml"); ok {
		t.Errorf("expected untracked %s/current.toml not in drift, got %+v", writ.Dir, report.Drift)
	}
}

func TestCompute_SimilarlyNamedPathsAreNotExcludedByPrefix(t *testing.T) {
	dir := newTestRepo(t)
	markBase(t, dir)
	writeFile(t, dir, ".writings/notes.md", "line one\nline two\n")
	writeFile(t, dir, ".writ-backup", "backup\n")

	report, err := Compute(testWrit("app/**"), dir)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if _, ok := findChange(report.Drift, ".writings/notes.md"); !ok {
		t.Errorf("expected .writings/notes.md to still be reported as drift, got %+v", report.Drift)
	}
	if _, ok := findChange(report.Drift, ".writ-backup"); !ok {
		t.Errorf("expected .writ-backup to still be reported as drift, got %+v", report.Drift)
	}
}

func TestCompute_NonExistentBaseReturnsError(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "app/foo.go", "package app\n")
	git(t, dir, "add", "app/foo.go")
	git(t, dir, "commit", "-q", "-m", "add foo")

	w := testWrit("app/**")
	w.Base = "does-not-exist"
	report, err := Compute(w, dir)
	if err == nil {
		t.Fatalf("expected error for non-existent base, got report %+v", report)
	}
	if report != nil {
		t.Errorf("expected nil report on error, got %+v", report)
	}
}
