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
	payload := struct {
		Security   string `json:"security,omitempty"`
		Platform   string `json:"platform,omitempty"`
		Visibility string `json:"visibility,omitempty"`
	}{}
	valid := true
	for _, path := range paths {
		doc, errs := read(path)
		for _, message := range errs {
			fmt.Fprintf(stderr, "%s: %s\n", path, message)
		}
		if len(errs) != 0 {
			valid = false
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "%s: can't be read: %s\n", path, reason(err))
			valid = false
			continue
		}
		view, _ := doc.Get("view")
		switch view {
		case "security":
			if payload.Security != "" {
				fmt.Fprintln(stderr, "awml push: two security views were given")
				valid = false
			} else {
				payload.Security = string(src)
			}
		case "platform":
			if payload.Platform != "" {
				fmt.Fprintln(stderr, "awml push: two platform views were given")
				valid = false
			} else {
				payload.Platform = string(src)
			}
		}
	}
	if !valid {
		return 1
	}
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

func pull(name, dir, registry string, stdout, stderr io.Writer) int {
	owner, slug, version, ok := pulledWorkflowName(name)
	if !ok {
		fmt.Fprintln(stderr, "awml pull: workflow must be owner/slug or owner/slug@n")
		return 2
	}
	token, err := credentialFor(registry)
	if err != nil {
		fmt.Fprintf(stderr, "awml pull: %s\n", err)
		return 1
	}
	if version == 0 {
		response, err := request(http.MethodGet, apiURL(registry, owner, slug), token, nil)
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
			fmt.Fprintln(stderr, "awml pull: the workflow has no versions")
			return 1
		}
	}
	endpoint := apiURL(registry, owner, slug) + "/v/" + strconv.Itoa(version)
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
	if downloaded.Files.Security == "" && downloaded.Files.Platform == "" {
		fmt.Fprintf(stderr, "awml pull: version %d of %s/%s came with no files\n", version, owner, slug)
		return 1
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "%s: can't be created: %s\n", dir, reason(err))
		return 1
	}
	for filename, source := range map[string]string{"security.yaml": downloaded.Files.Security, "platform.yaml": downloaded.Files.Platform} {
		if source == "" {
			continue
		}
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: can't be written: %s\n", path, reason(err))
			return 1
		}
	}
	fmt.Fprintf(stdout, "Pulled %s/%s version %d to %s\n", owner, slug, version, dir)
	return 0
}

func workflowName(name string) (string, string, bool) {
	owner, slug, ok := strings.Cut(name, "/")
	return owner, slug, ok && owner != "" && slug != "" && !strings.Contains(slug, "/")
}

func pulledWorkflowName(name string) (string, string, int, bool) {
	workflow, rawVersion, hasVersion := strings.Cut(name, "@")
	owner, slug, ok := workflowName(workflow)
	if !ok {
		return "", "", 0, false
	}
	if !hasVersion {
		return owner, slug, 0, true
	}
	version, err := strconv.Atoi(rawVersion)
	return owner, slug, version, err == nil && version > 0
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
