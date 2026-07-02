package ci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

type job struct {
	Steps []step `yaml:"steps"`
}

type step struct {
	Name string `yaml:"name"`
	If   string `yaml:"if"`
	Uses string `yaml:"uses"`
}

func TestCodecovUploadSkippedForDependabotPRs(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read ci workflow: %v", err)
	}

	var ci workflow
	if err := yaml.Unmarshal(data, &ci); err != nil {
		t.Fatalf("parse ci workflow: %v", err)
	}

	testJob, ok := ci.Jobs["test"]
	if !ok {
		t.Fatal("ci workflow is missing the test job")
	}

	var codecovStep *step
	for i := range testJob.Steps {
		if testJob.Steps[i].Name == "Upload coverage reports to Codecov" {
			codecovStep = &testJob.Steps[i]
			break
		}
	}
	if codecovStep == nil {
		t.Fatal("test job is missing the Codecov upload step")
	}
	if !strings.Contains(codecovStep.Uses, "codecov/codecov-action") {
		t.Fatalf("Codecov step uses %q, want codecov/codecov-action", codecovStep.Uses)
	}
	if !strings.Contains(codecovStep.If, "github.event_name != 'pull_request'") ||
		!strings.Contains(codecovStep.If, "dependabot[bot]") {
		t.Fatalf("Codecov upload should be skipped for Dependabot PRs, got if: %q", codecovStep.If)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root from test working directory")
		}
		dir = parent
	}
}
