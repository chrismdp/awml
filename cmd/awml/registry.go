package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chrismdp/awml"
)

const defaultRegistry = "https://awml.io"

// The registry to use: --registry, else AWML_REGISTRY_URL, else awml.io.
func registryURL(flag *string) string {
	registry := os.Getenv("AWML_REGISTRY_URL")
	if registry == "" {
		registry = defaultRegistry
	}
	if flag != nil {
		registry = *flag
	}
	return strings.TrimRight(strings.TrimSpace(registry), "/")
}

func login(stdin io.Reader, stdout, stderr io.Writer, registry string) int {
	if isTerminal(stdin) {
		fmt.Fprint(stderr, "Token: ")
	}
	token, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		fmt.Fprintf(stderr, "awml login: can't read the token: %s\n", err)
		return 1
	}
	token = strings.TrimSpace(token)
	if token == "" {
		fmt.Fprintln(stderr, "awml login: the token is empty")
		return 1
	}
	credentials, err := loadCredentials()
	if err != nil {
		fmt.Fprintf(stderr, "awml login: %s\n", err)
		return 1
	}
	credentials[registry] = token
	if err := writeCredentials(credentials); err != nil {
		fmt.Fprintf(stderr, "awml login: can't save the token: %s\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Logged in to %s\n", registry)
	return 0
}

func push(name string, paths []string, private bool, registry string, stdout, stderr io.Writer) int {
	owner, slug, ok := workflowName(name)
	if !ok {
		fmt.Fprintln(stderr, "awml push: workflow must be owner/slug")
		return 2
	}
	if len(paths) != 1 {
		printConvertAdvice(stderr)
		return 1
	}
	payload := struct {
		Workflow   string `json:"workflow"`
		Visibility string `json:"visibility,omitempty"`
	}{}
	given := sources(paths)
	workflows, err := awml.ReadSources(given)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !awml.IsCanonical(workflows[0].Doc) {
		printConvertAdvice(stderr)
		return 1
	}
	payload.Workflow = string(given[0].Text)
	if private {
		payload.Visibility = "private"
	}
	token, err := credentialFor(registry)
	if err != nil {
		fmt.Fprintf(stderr, "awml push: %s\n", err)
		return 1
	}
	if token == "" {
		fmt.Fprintf(stderr, "awml push: no token for %s; run awml login\n", registry)
		return 1
	}
	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(stderr, "awml push: can't make the request: %s\n", err)
		return 1
	}
	endpoint := apiURL(registry, owner, slug) + "/versions"
	response, err := request(http.MethodPost, endpoint, token, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "awml push: %s\n", err)
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		printAPIError(stderr, response)
		return 1
	}
	var pushed struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&pushed); err != nil {
		fmt.Fprintf(stderr, "awml push: the registry returned an unreadable response: %s\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Pushed %s version %d: %s\n", name, pushed.Number, pushed.URL)
	return 0
}

func printConvertAdvice(stderr io.Writer) {
	fmt.Fprintln(stderr, "awml push: push one 0.9 workflow; join an old pair with awml convert security.yaml platform.yaml > workflow.yaml")
}

func pull(name, dir, registry string, stdout, stderr io.Writer) int {
	owner, slug, platform, version, ok := pulledName(name)
	if !ok {
		fmt.Fprintln(stderr, "awml pull: workflow must be owner/slug or owner/slug@n, or a platform owner/platforms/slug or owner/platforms/slug@n")
		return 2
	}
	// A platform description lives beside the owner's workflows, under platforms.
	base, pulled := apiURL(registry, owner, slug), owner+"/"+slug
	if platform {
		base, pulled = registry+"/api/v1/"+url.PathEscape(owner)+"/platforms/"+url.PathEscape(slug), owner+"/platforms/"+slug
	}
	token, err := credentialFor(registry)
	if err != nil {
		fmt.Fprintf(stderr, "awml pull: %s\n", err)
		return 1
	}
	if version == 0 {
		response, err := request(http.MethodGet, base, token, nil)
		if err != nil {
			fmt.Fprintf(stderr, "awml pull: %s\n", err)
			return 1
		}
		if response.StatusCode != http.StatusOK {
			printAPIError(stderr, response)
			response.Body.Close()
			return 1
		}
		var workflow struct {
			Versions []struct {
				Number int `json:"number"`
			} `json:"versions"`
		}
		err = json.NewDecoder(response.Body).Decode(&workflow)
		response.Body.Close()
		if err != nil {
			fmt.Fprintf(stderr, "awml pull: the registry returned an unreadable workflow: %s\n", err)
			return 1
		}
		for _, candidate := range workflow.Versions {
			if candidate.Number > version {
				version = candidate.Number
			}
		}
		if version == 0 {
			if platform {
				fmt.Fprintln(stderr, "awml pull: the platform has no versions")
			} else {
				fmt.Fprintln(stderr, "awml pull: the workflow has no versions")
			}
			return 1
		}
	}
	endpoint := base + "/v/" + strconv.Itoa(version)
	response, err := request(http.MethodGet, endpoint, token, nil)
	if err != nil {
		fmt.Fprintf(stderr, "awml pull: %s\n", err)
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		printAPIError(stderr, response)
		return 1
	}
	var downloaded struct {
		Number int `json:"number"`
		Files  struct {
			Workflow string `json:"workflow"`
			Security string `json:"security"`
			Platform string `json:"platform"`
		} `json:"files"`
	}
	if err := json.NewDecoder(response.Body).Decode(&downloaded); err != nil {
		fmt.Fprintf(stderr, "awml pull: the registry returned an unreadable version: %s\n", err)
		return 1
	}
	if downloaded.Number != 0 {
		version = downloaded.Number
	}
	if downloaded.Files.Workflow == "" && downloaded.Files.Security == "" && downloaded.Files.Platform == "" {
		fmt.Fprintf(stderr, "awml pull: version %d of %s came with no files\n", version, pulled)
		return 1
	}
	if platform && downloaded.Files.Platform == "" {
		fmt.Fprintf(stderr, "awml pull: version %d of %s came with no platform description\n", version, pulled)
		return 1
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "%s: can't be created: %s\n", dir, reason(err))
		return 1
	}
	files := map[string]string{slug + ".yaml": downloaded.Files.Workflow}
	if platform {
		files = map[string]string{slug + ".yaml": downloaded.Files.Platform}
	} else if downloaded.Files.Workflow == "" {
		files = map[string]string{"security.yaml": downloaded.Files.Security, "platform.yaml": downloaded.Files.Platform}
	}
	for filename, source := range files {
		if source == "" {
			continue
		}
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: can't be written: %s\n", path, reason(err))
			return 1
		}
	}
	fmt.Fprintf(stdout, "Pulled %s version %d to %s\n", pulled, version, dir)
	return 0
}

func workflowName(name string) (string, string, bool) {
	owner, slug, ok := strings.Cut(name, "/")
	return owner, slug, ok && owner != "" && slug != "" && !strings.Contains(slug, "/")
}

// pulledName reads owner/slug or owner/platforms/slug, either with an optional @n, and says
// whether it names a platform description.
func pulledName(name string) (string, string, bool, int, bool) {
	named, rawVersion, hasVersion := strings.Cut(name, "@")
	platform := false
	if owner, rest, ok := strings.Cut(named, "/"); ok {
		if slug, isPlatform := strings.CutPrefix(rest, "platforms/"); isPlatform {
			named, platform = owner+"/"+slug, true
		}
	}
	owner, slug, ok := workflowName(named)
	if !ok {
		return "", "", false, 0, false
	}
	if !hasVersion {
		return owner, slug, platform, 0, true
	}
	version, err := strconv.Atoi(rawVersion)
	return owner, slug, platform, version, err == nil && version > 0
}

func apiURL(registry, owner, slug string) string {
	return registry + "/api/v1/" + url.PathEscape(owner) + "/" + url.PathEscape(slug)
}

func request(method, endpoint, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func printAPIError(stderr io.Writer, response *http.Response) {
	var problem struct {
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&problem); err == nil && len(problem.Errors) != 0 {
		for _, message := range problem.Errors {
			fmt.Fprintln(stderr, message)
		}
		return
	}
	fmt.Fprintf(stderr, "registry: %s\n", response.Status)
}

func credentialsPath() (string, error) {
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		config = filepath.Join(home, ".config")
	}
	return filepath.Join(config, "awml", "credentials"), nil
}

func loadCredentials() (map[string]string, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("can't read credentials: %w", err)
	}
	credentials := map[string]string{}
	if err := json.Unmarshal(data, &credentials); err != nil {
		return nil, fmt.Errorf("can't read credentials: %w", err)
	}
	return credentials, nil
}

func credentialFor(registry string) (string, error) {
	credentials, err := loadCredentials()
	if err != nil {
		return "", err
	}
	return credentials[registry], nil
}

func writeCredentials(credentials map[string]string) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// isTerminal says whether input comes from a person at a terminal, so a prompt only shows for one
// and a token piped in arrives silently.
func isTerminal(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
