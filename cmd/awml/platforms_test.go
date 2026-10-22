package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const pulledDescription = `awml: "0.5"
platform: { id: acme/platforms/ops-runner, label: "The ops runner" }
off_for_up_to: PT0S
steps: code
agents: []
calls: any
holds: []
`

func TestPushSendsAWorkflowNamingARegisteredPlatform(t *testing.T) {
	useTemporaryHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"owner":"acme","slug":"bread","number":1,"url":"/acme/bread/v/1"}`)
	}))
	defer server.Close()
	loginForTest(t, server.URL, "token")
	source := strings.Replace(strings.Replace(canonicalFile, `awml: "0.4"`, `awml: "0.5"`, 1), "platform: scripts", "platform: acme/platforms/ops-runner", 1)
	r := awmlRun("push", "acme/bread", write(t, "bread.yaml", source), "--registry", server.URL)
	if r != (result{0, "Pushed acme/bread version 1: /acme/bread/v/1\n", ""}) {
		t.Fatalf("push gave %+v", r)
	}
}

func TestPullDownloadsAPlatformDescription(t *testing.T) {
	useTemporaryHome(t)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/v/2") {
			fmt.Fprintf(w, `{"number":2,"files":{"platform":%q}}`, pulledDescription)
			return
		}
		fmt.Fprint(w, `{"versions":[{"number":1},{"number":2}]}`)
	}))
	defer server.Close()
	out := filepath.Join(t.TempDir(), "platforms")
	r := awmlRun("pull", "acme/platforms/ops-runner", "-o", out, "--registry", server.URL)
	if r != (result{0, "Pulled acme/platforms/ops-runner version 2 to " + out + "\n", ""}) {
		t.Fatalf("pull gave %+v", r)
	}
	if want := []string{"/api/v1/acme/platforms/ops-runner", "/api/v1/acme/platforms/ops-runner/v/2"}; !reflect.DeepEqual(paths, want) {
		t.Errorf("paths = %q, want %q", paths, want)
	}
	got, err := os.ReadFile(filepath.Join(out, "ops-runner.yaml"))
	if err != nil || string(got) != pulledDescription {
		t.Fatalf("ops-runner.yaml = %q, %v", got, err)
	}
	if r := awmlRun("check", filepath.Join(out, "ops-runner.yaml")); r.code != 0 {
		t.Errorf("check gave %+v", r)
	}
}

func TestPullDownloadsAPlatformVersion(t *testing.T) {
	useTemporaryHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/acme/platforms/ops-runner/v/1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprintf(w, `{"number":1,"files":{"platform":%q}}`, pulledDescription)
	}))
	defer server.Close()
	out := t.TempDir()
	r := awmlRun("pull", "acme/platforms/ops-runner@1", "-o", out, "--registry", server.URL)
	if r != (result{0, "Pulled acme/platforms/ops-runner version 1 to " + out + "\n", ""}) {
		t.Fatalf("pull gave %+v", r)
	}
	if _, err := os.Stat(filepath.Join(out, "ops-runner.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestPullRefusesAnotherThreePartName(t *testing.T) {
	r := awmlRun("pull", "acme/workflows/ops-runner")
	if r.code != 2 || r.stderr != "awml pull: workflow must be owner/slug or owner/slug@n, or a platform owner/platforms/slug or owner/platforms/slug@n\n" {
		t.Fatalf("pull gave %+v", r)
	}
}

// A block that names a registered platform not given to awml fit says how to pull it.
func TestFitHintsAtPullingARegisteredPlatform(t *testing.T) {
	source := strings.Replace(strings.Replace(canonicalFile, `awml: "0.4"`, `awml: "0.5"`, 1), "platform: scripts", "platform: acme/platforms/ops-runner", 1)
	workflow := write(t, "bread.yaml", source)
	other := write(t, "other.yaml", strings.Replace(pulledDescription, "acme/platforms/ops-runner", "other", 1))
	r := awmlRun("fit", "--implementation", "home", workflow, other)
	if r.code != 1 || !strings.Contains(r.stdout, `group "server" names platform "acme/platforms/ops-runner", but its description wasn't given: pull it with awml pull acme/platforms/ops-runner`) {
		t.Fatalf("fit gave %+v", r)
	}
	pulled := write(t, "ops-runner.yaml", pulledDescription)
	if r := awmlRun("fit", "--implementation", "home", workflow, pulled); r != (result{0, "server  fits: plan may use any\n", ""}) {
		t.Fatalf("fit gave %+v", r)
	}
}
