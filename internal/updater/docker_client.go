package updater

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// DockerClient is a lightweight Docker Engine HTTP API client over a Unix domain socket.
type DockerClient struct {
	sockPath   string
	baseURL    string
	httpClient *http.Client
}

// NewDockerClient creates a Docker Engine client bound to the given Unix socket path.
func NewDockerClient(sockPath string) *DockerClient {
	if strings.TrimSpace(sockPath) == "" {
		sockPath = "/var/run/docker.sock"
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sockPath)
		},
	}
	return &DockerClient{
		sockPath: sockPath,
		baseURL:  "http://docker",
		httpClient: &http.Client{
			Transport: transport,
		},
	}
}

// NewDockerClientWithHTTP creates a DockerClient backed by a custom base URL and HTTP client (for tests).
func NewDockerClientWithHTTP(baseURL string, hc *http.Client) *DockerClient {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &DockerClient{
		sockPath:   "/var/run/docker.sock",
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: hc,
	}
}

type ContainerInspect struct {
	ID              string           `json:"Id"`
	Name            string           `json:"Name"`
	Image           string           `json:"Image"`
	Config          *ContainerConfig `json:"Config"`
	HostConfig      map[string]any   `json:"HostConfig"`
	Mounts          []ContainerMount `json:"Mounts"`
	NetworkSettings *NetworkSettings `json:"NetworkSettings"`
}

type ContainerConfig struct {
	Hostname     string              `json:"Hostname,omitempty"`
	Domainname   string              `json:"Domainname,omitempty"`
	User         string              `json:"User,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Image        string              `json:"Image,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	StopSignal   string              `json:"StopSignal,omitempty"`
	StopTimeout  *int                `json:"StopTimeout,omitempty"`
}

type ContainerMount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
}

type NetworkSettings struct {
	Networks map[string]*EndpointSettings `json:"Networks"`
}

type EndpointSettings struct {
	IPAMConfig *EndpointIPAMConfig `json:"IPAMConfig,omitempty"`
	Links      []string            `json:"Links,omitempty"`
	Aliases    []string            `json:"Aliases,omitempty"`
}

type EndpointIPAMConfig struct {
	IPv4Address string `json:"IPv4Address,omitempty"`
	IPv6Address string `json:"IPv6Address,omitempty"`
}

type ImageInspect struct {
	ID          string   `json:"Id"`
	RepoTags    []string `json:"RepoTags"`
	RepoDigests []string `json:"RepoDigests"`
}

type dockerErrorResponse struct {
	Message string `json:"message"`
}

func (c *DockerClient) doJSON(ctx context.Context, method, path string, body any, out any) (int, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		var derr dockerErrorResponse
		if json.Unmarshal(raw, &derr) == nil && strings.TrimSpace(derr.Message) != "" {
			return resp.StatusCode, fmt.Errorf("docker API %s %s (HTTP %d): %s", method, path, resp.StatusCode, derr.Message)
		}
		return resp.StatusCode, fmt.Errorf("docker API %s %s (HTTP %d): %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode docker response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func (c *DockerClient) Ping(ctx context.Context) error {
	_, err := c.doJSON(ctx, http.MethodGet, "/_ping", nil, nil)
	return err
}

func (c *DockerClient) InspectContainer(ctx context.Context, id string) (*ContainerInspect, error) {
	var out ContainerInspect
	_, err := c.doJSON(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *DockerClient) InspectImage(ctx context.Context, id string) (*ImageInspect, error) {
	var out ImageInspect
	_, err := c.doJSON(ctx, http.MethodGet, "/images/"+url.PathEscape(id)+"/json", nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// PullImage pulls a Docker image reference (repo[:tag]) and streams NDJSON status updates.
func (c *DockerClient) PullImage(ctx context.Context, imageRef string, onProgress func(string)) error {
	repo, tag := SplitImageRef(imageRef)
	if repo == "" {
		return errors.New("empty image repository")
	}
	q := url.Values{}
	q.Set("fromImage", repo)
	if tag != "" {
		q.Set("tag", tag)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/images/create?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var derr dockerErrorResponse
		if json.Unmarshal(raw, &derr) == nil && strings.TrimSpace(derr.Message) != "" {
			return fmt.Errorf("pull %s (HTTP %d): %s", imageRef, resp.StatusCode, derr.Message)
		}
		return fmt.Errorf("pull %s (HTTP %d): %s", imageRef, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev struct {
			Status      string `json:"status"`
			ID          string `json:"id"`
			Progress    string `json:"progress"`
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.ErrorDetail.Message != "" {
			return fmt.Errorf("pull %s: %s", imageRef, ev.ErrorDetail.Message)
		}
		if ev.Error != "" {
			return fmt.Errorf("pull %s: %s", imageRef, ev.Error)
		}
		if onProgress != nil && ev.Status != "" {
			msg := ev.Status
			if ev.ID != "" {
				msg = ev.ID + ": " + msg
			}
			if ev.Progress != "" {
				msg += " " + ev.Progress
			}
			onProgress(msg)
		}
	}
	return scanner.Err()
}

func (c *DockerClient) CreateContainer(ctx context.Context, name string, payload map[string]any) (string, error) {
	path := "/containers/create"
	if strings.TrimSpace(name) != "" {
		path += "?name=" + url.QueryEscape(strings.TrimPrefix(name, "/"))
	}
	var res struct {
		ID string `json:"Id"`
	}
	if _, err := c.doJSON(ctx, http.MethodPost, path, payload, &res); err != nil {
		return "", err
	}
	if res.ID == "" {
		return "", errors.New("docker create container returned empty ID")
	}
	return res.ID, nil
}

func (c *DockerClient) StartContainer(ctx context.Context, id string) error {
	_, err := c.doJSON(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil, nil)
	return err
}

func (c *DockerClient) StopContainer(ctx context.Context, id string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	status, err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/containers/%s/stop?t=%d", url.PathEscape(id), timeoutSec), nil, nil)
	if status == http.StatusNotModified {
		return nil
	}
	return err
}

func (c *DockerClient) RenameContainer(ctx context.Context, id, newName string) error {
	path := fmt.Sprintf("/containers/%s/rename?name=%s", url.PathEscape(id), url.QueryEscape(strings.TrimPrefix(newName, "/")))
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, nil)
	return err
}

func (c *DockerClient) RemoveContainer(ctx context.Context, id string, force bool) error {
	q := ""
	if force {
		q = "?force=true"
	}
	_, err := c.doJSON(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+q, nil, nil)
	return err
}

// SplitImageRef splits an image reference like "ghcr.io/owner/repo:v1.0.0" into repository and tag.
// If no tag is present, tag defaults to "latest".
func SplitImageRef(ref string) (repo, tag string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	// Strip digest if present (e.g. repo:tag@sha256:...)
	if at := strings.IndexByte(ref, '@'); at >= 0 {
		ref = ref[:at]
	}
	lastSlash := strings.LastIndexByte(ref, '/')
	lastColon := strings.LastIndexByte(ref, ':')
	if lastColon > lastSlash {
		return ref[:lastColon], ref[lastColon+1:]
	}
	return ref, "latest"
}

var (
	reMountContainerID  = regexp.MustCompile(`(?:/docker/containers/|/containers/)([0-9a-f]{64})/`)
	reCgroupContainerID = regexp.MustCompile(`(?:docker-|docker/|containerd-|cri-containerd-|libpod-)([0-9a-f]{64})`)
	reValidImageRef     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{1,254}$`)
)

// IsValidImageRef validates a Docker image reference for safe API calls.
func IsValidImageRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if !reValidImageRef.MatchString(ref) {
		return false
	}
	if strings.Contains(ref, "..") || strings.ContainsAny(ref, " \t\r\n") {
		return false
	}
	return true
}

// IsInDocker reports whether the current process appears to be running inside a Docker container.
func IsInDocker() bool {
	if strings.TrimSpace(os.Getenv("WALRUS_CONTAINER_ID")) != "" {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		if reMountContainerID.Match(data) {
			return true
		}
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		if strings.Contains(s, "docker") || strings.Contains(s, "containerd") || strings.Contains(s, "kubepods") {
			return true
		}
	}
	return false
}

// DetectSelfContainerID attempts to determine the current Docker container ID.
func DetectSelfContainerID() string {
	if envID := strings.TrimSpace(os.Getenv("WALRUS_CONTAINER_ID")); envID != "" {
		return envID
	}
	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		if m := reMountContainerID.FindSubmatch(data); len(m) == 2 {
			return string(m[1])
		}
	}
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		if m := reCgroupContainerID.FindSubmatch(data); len(m) == 2 {
			return string(m[1])
		}
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		if hn, err := os.Hostname(); err == nil && strings.TrimSpace(hn) != "" {
			return strings.TrimSpace(hn)
		}
	}
	return ""
}

// ResolveHostDockerSock finds the host-side path mounted to containerSock inside self.
func ResolveHostDockerSock(self *ContainerInspect, containerSock string) string {
	if self != nil {
		for _, m := range self.Mounts {
			if m.Destination == containerSock && strings.TrimSpace(m.Source) != "" {
				return m.Source
			}
		}
	}
	return containerSock
}

// BuildRecreatePayload converts an inspected container into a Docker /containers/create payload
// pointing to targetImage while preserving env, labels, mounts/binds, ports, restart policy, and networks.
func BuildRecreatePayload(self *ContainerInspect, targetImage string) map[string]any {
	payload := map[string]any{
		"Image": targetImage,
	}

	if self != nil && self.Config != nil {
		cfg := self.Config
		// Only preserve Hostname if it wasn't the auto-generated 12-char container ID
		if cfg.Hostname != "" && !(len(self.ID) >= 12 && cfg.Hostname == self.ID[:12]) {
			payload["Hostname"] = cfg.Hostname
		}
		if cfg.Domainname != "" {
			payload["Domainname"] = cfg.Domainname
		}
		if cfg.User != "" {
			payload["User"] = cfg.User
		}
		if len(cfg.Env) > 0 {
			payload["Env"] = cfg.Env
		}
		if len(cfg.Cmd) > 0 {
			payload["Cmd"] = cfg.Cmd
		}
		if len(cfg.Volumes) > 0 {
			payload["Volumes"] = cfg.Volumes
		}
		if cfg.WorkingDir != "" {
			payload["WorkingDir"] = cfg.WorkingDir
		}
		if len(cfg.Labels) > 0 {
			payload["Labels"] = cfg.Labels
		}
		if len(cfg.ExposedPorts) > 0 {
			payload["ExposedPorts"] = cfg.ExposedPorts
		}
		if cfg.StopSignal != "" {
			payload["StopSignal"] = cfg.StopSignal
		}
		if cfg.StopTimeout != nil {
			payload["StopTimeout"] = *cfg.StopTimeout
		}
	}

	var netMode string
	if self != nil && len(self.HostConfig) > 0 {
		hc := make(map[string]any, len(self.HostConfig))
		for k, v := range self.HostConfig {
			hc[k] = v
		}
		if nm, ok := hc["NetworkMode"].(string); ok {
			netMode = nm
		}
		payload["HostConfig"] = hc
	}

	// Preserve user-defined bridge network endpoints when not using host/none/container networking.
	if self != nil && self.NetworkSettings != nil && len(self.NetworkSettings.Networks) > 0 &&
		netMode != "host" && netMode != "none" && !strings.HasPrefix(netMode, "container:") {
		endpoints := make(map[string]any, len(self.NetworkSettings.Networks))
		for netName, ep := range self.NetworkSettings.Networks {
			if ep == nil {
				continue
			}
			epCfg := map[string]any{}
			if len(ep.Aliases) > 0 {
				// Filter out old container short ID from aliases so Docker generates a new one.
				filtered := make([]string, 0, len(ep.Aliases))
				for _, a := range ep.Aliases {
					if len(self.ID) >= 12 && a == self.ID[:12] {
						continue
					}
					filtered = append(filtered, a)
				}
				if len(filtered) > 0 {
					epCfg["Aliases"] = filtered
				}
			}
			if ep.IPAMConfig != nil && (ep.IPAMConfig.IPv4Address != "" || ep.IPAMConfig.IPv6Address != "") {
				epCfg["IPAMConfig"] = ep.IPAMConfig
			}
			if len(ep.Links) > 0 {
				epCfg["Links"] = ep.Links
			}
			endpoints[netName] = epCfg
		}
		if len(endpoints) > 0 {
			payload["NetworkingConfig"] = map[string]any{
				"EndpointsConfig": endpoints,
			}
		}
	}

	return payload
}
