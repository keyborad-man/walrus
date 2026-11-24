package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"walrus/internal/apperr"
	"walrus/internal/version"
)

// DockerCapability describes whether the running Walrus instance can self-upgrade via Docker Engine API.
type DockerCapability struct {
	InDocker           bool   `json:"in_docker"`
	SocketAvailable    bool   `json:"socket_available"`
	CanSelfUpgrade     bool   `json:"can_self_upgrade"`
	ContainerID        string `json:"container_id,omitempty"`
	ContainerName      string `json:"container_name,omitempty"`
	CurrentImage       string `json:"current_image,omitempty"`
	TargetImage        string `json:"target_image,omitempty"`
	TargetImagePending bool   `json:"target_image_pending,omitempty"`
	UnavailableReason  string `json:"unavailable_reason,omitempty"` // not_docker | socket_not_mounted | container_not_found
}

// UpgradeState describes the live status of an in-progress or recently attempted self-upgrade.
type UpgradeState struct {
	State       string `json:"state"` // idle | pulling | restarting | failed
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
	TargetImage string `json:"target_image,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
}

// VersionStatus is the full version, update-check, and Docker self-upgrade snapshot returned by the API.
type VersionStatus struct {
	CurrentVersion  string           `json:"current_version"`
	Commit          string           `json:"commit"`
	BuildTime       string           `json:"build_time"`
	GoVersion       string           `json:"go_version"`
	OS              string           `json:"os"`
	Arch            string           `json:"arch"`
	LatestVersion   string           `json:"latest_version,omitempty"`
	UpdateAvailable bool             `json:"update_available"`
	Source          string           `json:"source,omitempty"` // github_release | github_tag | ghcr_latest | none
	ReleaseURL      string           `json:"release_url,omitempty"`
	ReleaseNotes    string           `json:"release_notes,omitempty"`
	PublishedAt     string           `json:"published_at,omitempty"`
	LocalDigest     string           `json:"local_digest,omitempty"`
	RemoteDigest    string           `json:"remote_digest,omitempty"`
	LastCheckedAt   string           `json:"last_checked_at,omitempty"`
	LastCheckError  string           `json:"last_check_error,omitempty"`
	Docker          DockerCapability `json:"docker"`
	Upgrade         UpgradeState     `json:"upgrade"`
}

// Service manages Walrus version checking (GitHub Releases + GHCR fallback) and Docker self-upgrade.
type Service struct {
	mu             sync.RWMutex
	githubRepo     string
	imageRepo      string
	dockerSock     string
	runtimeMode    string
	httpClient     *http.Client
	dockerClient   *DockerClient
	githubAPIBase  string
	ghcrBase       string
	detectInDocker func() bool
	detectSelfID   func() string
	currentInfo    func() version.Info

	// Cached remote check result
	latestVersion      string
	updateAvailable    bool
	targetImagePending bool
	source             string
	releaseURL         string
	releaseNotes       string
	publishedAt        string
	localDigest        string
	remoteDigest       string
	lastCheckedAt      string
	lastCheckError     string

	// Live upgrade progress
	upgrade UpgradeState
}

// NewService creates a new Walrus version and self-upgrade service.
func NewService(githubRepo, imageRepo, dockerSock, runtimeMode string) *Service {
	info := version.Current()
	if strings.TrimSpace(githubRepo) == "" {
		githubRepo = info.GitHubRepo
	}
	if strings.TrimSpace(imageRepo) == "" {
		imageRepo = info.ImageRepo
	}
	if strings.TrimSpace(dockerSock) == "" {
		dockerSock = "/var/run/docker.sock"
	}
	return &Service{
		githubRepo:     strings.TrimSpace(githubRepo),
		imageRepo:      strings.TrimSpace(imageRepo),
		dockerSock:     strings.TrimSpace(dockerSock),
		runtimeMode:    runtimeMode,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		dockerClient:   NewDockerClient(dockerSock),
		githubAPIBase:  "https://api.github.com",
		ghcrBase:       "https://ghcr.io",
		detectInDocker: IsInDocker,
		detectSelfID:   DetectSelfContainerID,
		currentInfo:    version.Current,
		upgrade:        UpgradeState{State: "idle"},
	}
}

// Status returns the current version snapshot, refreshing Docker capability probes.
func (s *Service) Status(ctx context.Context) VersionStatus {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	capInfo, localDigest := s.probeDocker(probeCtx)

	s.mu.Lock()
	if localDigest != "" {
		s.localDigest = localDigest
	}
	st := s.buildStatusLocked(capInfo)
	s.mu.Unlock()
	return st
}

// CheckUpdate queries GitHub Releases (with GitHub Tags and GHCR latest digest fallback)
// and updates the cached update status.
func (s *Service) CheckUpdate(ctx context.Context) (VersionStatus, error) {
	info := s.currentInfo()
	probeCtx, cancelProbe := context.WithTimeout(ctx, 2*time.Second)
	capInfo, localDigest := s.probeDocker(probeCtx)
	cancelProbe()

	var (
		latestVer          string
		source             string
		releaseURL         string
		releaseNote        string
		publishedAt        string
		remoteDig          string
		updateAvail        bool
		targetImagePending bool
		errs               []string
	)

	// 1. Primary source: GitHub Releases (/releases/latest)
	rel, statusCode, err := s.fetchGitHubLatestRelease(ctx)
	if err == nil && rel != nil && strings.TrimSpace(rel.TagName) != "" {
		latestVer = strings.TrimSpace(rel.TagName)
		source = "github_release"
		releaseURL = strings.TrimSpace(rel.HTMLURL)
		releaseNote = strings.TrimSpace(rel.Body)
		publishedAt = strings.TrimSpace(rel.PublishedAt)
	} else {
		if err != nil && statusCode != http.StatusNotFound {
			errs = append(errs, "GitHub release: "+err.Error())
		}
		// 2. Secondary source: GitHub Tags (/tags) when no Release object exists yet
		if tag, tagErr := s.fetchGitHubLatestSemverTag(ctx); tagErr == nil && tag != "" {
			latestVer = tag
			source = "github_tag"
			releaseURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", s.githubRepo, tag)
		} else if tagErr != nil {
			errs = append(errs, "GitHub tags: "+tagErr.Error())
		}
	}

	// Evaluate semver comparison when we found a release/tag and current version is semver.
	if latestVer != "" {
		if cmp, ok := version.CompareSemver(info.Version, latestVer); ok {
			updateAvail = cmp < 0
		}
	}

	// 2.5. When running in Docker and a newer GitHub Release/Tag was found, verify the target
	// GHCR image is already published so we do not trigger 404 pulls while CI is still building.
	if updateAvail && capInfo.InDocker && (source == "github_release" || source == "github_tag") {
		targetRef := s.computeTargetImage(capInfo, source, latestVer)
		targetRepo, targetTag := SplitImageRef(targetRef)
		if strings.HasPrefix(strings.ToLower(targetRepo), "ghcr.io/") {
			dig, readyErr := s.fetchGHCRDigestWithTagFallback(ctx, targetRepo, targetTag)
			if readyErr == nil && dig != "" {
				remoteDig = dig
			} else if errors.Is(readyErr, errGHCRManifestNotFound) {
				targetImagePending = true
				updateAvail = false
			}
		}
	}

	// 3. GHCR latest digest fallback:
	//    - Always run when no GitHub Release/Tag was found, OR
	//    - Run when current version is non-semver (e.g. "dev" / "main-abc1234") so :latest deployments still detect updates.
	if latestVer == "" || !version.IsSemver(info.Version) {
		imgRepo := s.resolveImageRepo(capInfo.CurrentImage)
		_, tag := SplitImageRef(capInfo.CurrentImage)
		if tag == "" {
			tag = "latest"
		}
		if dig, digErr := s.fetchGHCRDigest(ctx, imgRepo, tag); digErr == nil && dig != "" {
			remoteDig = dig
			if latestVer == "" {
				latestVer = tag
				source = "ghcr_latest"
				releaseURL = fmt.Sprintf("https://github.com/%s/pkgs/container/%s", s.githubRepo, repoBaseName(imgRepo))
			}
			if localDigest != "" && !strings.EqualFold(localDigest, remoteDig) {
				updateAvail = true
			}
		} else if digErr != nil && latestVer == "" {
			errs = append(errs, "GHCR digest: "+digErr.Error())
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	s.mu.Lock()
	s.lastCheckedAt = now
	if localDigest != "" {
		s.localDigest = localDigest
	}
	if source != "" {
		s.latestVersion = latestVer
		s.source = source
		s.releaseURL = releaseURL
		s.releaseNotes = releaseNote
		s.publishedAt = publishedAt
		s.remoteDigest = remoteDig
		s.updateAvailable = updateAvail
		s.targetImagePending = targetImagePending
		s.lastCheckError = ""
	} else {
		s.source = "none"
		s.targetImagePending = false
		s.lastCheckError = strings.Join(errs, "; ")
	}
	st := s.buildStatusLocked(capInfo)
	s.mu.Unlock()

	if source == "" && len(errs) > 0 {
		return st, apperr.WrapDetail(apperr.CodeVersionCheckFailed, errors.New(strings.Join(errs, "; ")), "version check failed: %s", strings.Join(errs, "; "))
	}
	return st, nil
}

// StartUpgrade begins an asynchronous Docker image pull and container self-replacement.
func (s *Service) StartUpgrade(ctx context.Context, requestedImage string) (VersionStatus, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	capInfo, localDigest := s.probeDocker(probeCtx)
	cancel()

	if !capInfo.CanSelfUpgrade {
		return s.Status(ctx), apperr.Newf(
			apperr.CodeUpgradeUnavailable,
			map[string]any{"reason": capInfo.UnavailableReason},
			"docker one-click upgrade unavailable (%s)",
			capInfo.UnavailableReason,
		)
	}

	s.mu.Lock()
	if localDigest != "" {
		s.localDigest = localDigest
	}
	if s.upgrade.State == "pulling" || s.upgrade.State == "restarting" {
		st := s.buildStatusLocked(capInfo)
		s.mu.Unlock()
		return st, apperr.New(apperr.CodeUpgradeInProgress, "an upgrade is already in progress")
	}

	targetImage := strings.TrimSpace(requestedImage)
	if targetImage == "" {
		targetImage = s.computeTargetImageLocked(capInfo)
	}
	if !IsValidImageRef(targetImage) {
		s.mu.Unlock()
		return s.Status(ctx), apperr.New(apperr.CodeInvalidTargetImage, "invalid target docker image reference")
	}

	s.upgrade = UpgradeState{
		State:       "pulling",
		Message:     "Pulling " + targetImage + "...",
		TargetImage: targetImage,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	st := s.buildStatusLocked(capInfo)
	s.mu.Unlock()

	go s.runUpgradeFlow(capInfo.ContainerID, targetImage)
	return st, nil
}

func (s *Service) runUpgradeFlow(selfID, targetImage string) {
	fail := func(msg string, err error) {
		slog.Error("walrus docker upgrade failed", "step", msg, "target_image", targetImage, "err", err)
		s.mu.Lock()
		s.upgrade.State = "failed"
		s.upgrade.Message = msg
		if err != nil {
			s.upgrade.Error = err.Error()
		}
		s.mu.Unlock()
	}

	// 1. Pull target image (with graceful tag fallback if vX.Y.Z isn't published yet).
	pullCtx, cancelPull := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelPull()

	pulledImage, err := s.pullWithFallback(pullCtx, targetImage, func(progress string) {
		s.mu.Lock()
		if s.upgrade.State == "pulling" {
			s.upgrade.Message = progress
		}
		s.mu.Unlock()
	})
	if err != nil {
		fail("Failed to pull target image", err)
		return
	}

	// 2. Inspect self container to capture exact runtime config, mounts, ports, networks.
	inspCtx, cancelInsp := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInsp()
	self, err := s.dockerClient.InspectContainer(inspCtx, selfID)
	if err != nil {
		fail("Failed to inspect current container", err)
		return
	}

	containerName := strings.TrimPrefix(self.Name, "/")
	if containerName == "" {
		containerName = "walrus"
	}

	recreatePayload := BuildRecreatePayload(self, pulledImage)
	spec := UpgradeSpec{
		OldContainerID: self.ID,
		ContainerName:  containerName,
		TargetImage:    pulledImage,
		DockerSock:     s.dockerSock,
		CreatePayload:  recreatePayload,
	}
	encodedSpec, err := encodeUpgradeSpec(spec)
	if err != nil {
		fail("Failed to encode upgrade specification", err)
		return
	}

	hostSock := ResolveHostDockerSock(self, s.dockerSock)
	helperName := fmt.Sprintf("walrus-upgrader-%d", time.Now().Unix())
	// Reuse the current container's image for the helper so `--self-upgrade-helper` is guaranteed to exist.
	helperImage := self.Image
	if helperImage == "" && self.Config != nil {
		helperImage = self.Config.Image
	}

	helperPayload := map[string]any{
		"Image":      helperImage,
		"Entrypoint": []string{"/usr/local/bin/walrus"},
		"Cmd":        []string{"--self-upgrade-helper"},
		"Env": []string{
			upgradeSpecEnvKey + "=" + encodedSpec,
		},
		"HostConfig": map[string]any{
			"Binds":      []string{hostSock + ":" + s.dockerSock},
			"AutoRemove": true,
		},
	}

	createCtx, cancelCreate := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelCreate()
	helperID, err := s.dockerClient.CreateContainer(createCtx, helperName, helperPayload)
	if err != nil {
		fail("Failed to create temporary upgrade helper container", err)
		return
	}

	s.mu.Lock()
	s.upgrade.State = "restarting"
	s.upgrade.TargetImage = pulledImage
	s.upgrade.Message = "Gracefully stopping server and recreating container..."
	s.mu.Unlock()

	startCtx, cancelStart := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelStart()
	if err := s.dockerClient.StartContainer(startCtx, helperID); err != nil {
		_ = s.dockerClient.RemoveContainer(context.Background(), helperID, true)
		fail("Failed to start temporary upgrade helper container", err)
		return
	}

	slog.Info("walrus upgrade helper started; waiting for graceful SIGTERM swap",
		"helper_id", helperID,
		"target_image", pulledImage,
	)
}

func (s *Service) pullWithFallback(ctx context.Context, targetImage string, onProgress func(string)) (string, error) {
	err := s.dockerClient.PullImage(ctx, targetImage, onProgress)
	if err == nil {
		return targetImage, nil
	}
	isNotFound := strings.Contains(strings.ToLower(err.Error()), "not found")
	// If tag has leading "v" (e.g. :v0.2.0) and registry returned not found, also try :0.2.0.
	repo, tag := SplitImageRef(targetImage)
	if strings.HasPrefix(tag, "v") && version.IsSemver(tag) && isNotFound {
		alt := repo + ":" + strings.TrimPrefix(tag, "v")
		if altErr := s.dockerClient.PullImage(ctx, alt, onProgress); altErr == nil {
			return alt, nil
		}
	}
	if isNotFound {
		return "", fmt.Errorf("image %s is not yet published in registry (CI build may still be in progress; retry in 1-2 minutes): %w", targetImage, err)
	}
	return "", err
}

func (s *Service) probeDocker(ctx context.Context) (DockerCapability, string) {
	capInfo := DockerCapability{
		InDocker: s.detectInDocker(),
	}
	if !capInfo.InDocker {
		capInfo.UnavailableReason = "not_docker"
		return capInfo, ""
	}

	// Quick stat check on Unix socket if using default local client (skip when using HTTP test server).
	if strings.HasPrefix(s.dockerClient.baseURL, "http://docker") {
		if _, err := os.Stat(s.dockerSock); err != nil {
			capInfo.UnavailableReason = "socket_not_mounted"
			return capInfo, ""
		}
	}

	if err := s.dockerClient.Ping(ctx); err != nil {
		capInfo.UnavailableReason = "socket_not_mounted"
		return capInfo, ""
	}
	capInfo.SocketAvailable = true

	selfID := s.detectSelfID()
	if selfID == "" {
		capInfo.UnavailableReason = "container_not_found"
		return capInfo, ""
	}
	insp, err := s.dockerClient.InspectContainer(ctx, selfID)
	if err != nil {
		capInfo.UnavailableReason = "container_not_found"
		return capInfo, ""
	}

	capInfo.ContainerID = insp.ID
	capInfo.ContainerName = strings.TrimPrefix(insp.Name, "/")
	if insp.Config != nil {
		capInfo.CurrentImage = insp.Config.Image
	}
	capInfo.CanSelfUpgrade = true

	var localDigest string
	if insp.Image != "" {
		if img, err := s.dockerClient.InspectImage(ctx, insp.Image); err == nil && img != nil {
			for _, rd := range img.RepoDigests {
				if at := strings.IndexByte(rd, '@'); at >= 0 {
					localDigest = rd[at+1:]
					break
				}
			}
		}
	}
	return capInfo, localDigest
}

func (s *Service) resolveImageRepo(currentImage string) string {
	if repo, _ := SplitImageRef(currentImage); repo != "" {
		return repo
	}
	return s.imageRepo
}

func (s *Service) computeTargetImageLocked(capInfo DockerCapability) string {
	return s.computeTargetImage(capInfo, s.source, s.latestVersion)
}

func (s *Service) computeTargetImage(capInfo DockerCapability, source, latestVersion string) string {
	repo := s.resolveImageRepo(capInfo.CurrentImage)
	_, currentTag := SplitImageRef(capInfo.CurrentImage)
	if currentTag == "" {
		currentTag = "latest"
	}
	// If we discovered a semver release/tag, target the explicit versioned image tag.
	if (source == "github_release" || source == "github_tag") && version.IsSemver(latestVersion) {
		if currentTag == "latest" || currentTag == "main" {
			return repo + ":" + latestVersion
		}
		if strings.HasPrefix(currentTag, "v") {
			if strings.HasPrefix(latestVersion, "v") {
				return repo + ":" + latestVersion
			}
			return repo + ":v" + latestVersion
		}
		return repo + ":" + strings.TrimPrefix(latestVersion, "v")
	}
	return repo + ":" + currentTag
}

func (s *Service) buildStatusLocked(capInfo DockerCapability) VersionStatus {
	info := s.currentInfo()
	capInfo.TargetImage = s.computeTargetImageLocked(capInfo)
	capInfo.TargetImagePending = s.targetImagePending
	return VersionStatus{
		CurrentVersion:  info.Version,
		Commit:          info.Commit,
		BuildTime:       info.BuildTime,
		GoVersion:       info.GoVersion,
		OS:              info.OS,
		Arch:            info.Arch,
		LatestVersion:   s.latestVersion,
		UpdateAvailable: s.updateAvailable,
		Source:          s.source,
		ReleaseURL:      s.releaseURL,
		ReleaseNotes:    s.releaseNotes,
		PublishedAt:     s.publishedAt,
		LocalDigest:     s.localDigest,
		RemoteDigest:    s.remoteDigest,
		LastCheckedAt:   s.lastCheckedAt,
		LastCheckError:  s.lastCheckError,
		Docker:          capInfo,
		Upgrade:         s.upgrade,
	}
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

func (s *Service) fetchGitHubLatestRelease(ctx context.Context) (*githubRelease, int, error) {
	if s.githubRepo == "" {
		return nil, 0, errors.New("github repo not configured")
	}
	u := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(s.githubAPIBase, "/"), s.githubRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Walrus/"+s.currentInfo().Version)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, resp.StatusCode, err
	}
	return &rel, resp.StatusCode, nil
}

func (s *Service) fetchGitHubLatestSemverTag(ctx context.Context) (string, error) {
	if s.githubRepo == "" {
		return "", errors.New("github repo not configured")
	}
	u := fmt.Sprintf("%s/repos/%s/tags?per_page=20", strings.TrimRight(s.githubAPIBase, "/"), s.githubRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Walrus/"+s.currentInfo().Version)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&tags); err != nil {
		return "", err
	}
	best := ""
	for _, t := range tags {
		name := strings.TrimSpace(t.Name)
		if !version.IsSemver(name) {
			continue
		}
		if best == "" {
			best = name
			continue
		}
		if cmp, ok := version.CompareSemver(best, name); ok && cmp < 0 {
			best = name
		}
	}
	return best, nil
}

var errGHCRManifestNotFound = errors.New("ghcr manifest not found")

func (s *Service) fetchGHCRDigestWithTagFallback(ctx context.Context, imageRepo, tag string) (string, error) {
	dig, err := s.fetchGHCRDigest(ctx, imageRepo, tag)
	if err == nil {
		return dig, nil
	}
	if errors.Is(err, errGHCRManifestNotFound) && strings.HasPrefix(tag, "v") && version.IsSemver(tag) {
		return s.fetchGHCRDigest(ctx, imageRepo, strings.TrimPrefix(tag, "v"))
	}
	return "", err
}

func (s *Service) fetchGHCRDigest(ctx context.Context, imageRepo, tag string) (string, error) {
	// Extract owner/repo from ghcr.io/owner/repo
	repoPath := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(imageRepo)), "ghcr.io/")
	if repoPath == "" || strings.Contains(repoPath, "://") || !strings.Contains(repoPath, "/") {
		return "", errors.New("unsupported registry for digest check")
	}
	if strings.TrimSpace(tag) == "" {
		tag = "latest"
	}

	// Step 1: Anonymous GHCR pull token
	tokenURL := fmt.Sprintf("%s/token?scope=repository:%s:pull", strings.TrimRight(s.ghcrBase, "/"), repoPath)
	tReq, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", err
	}
	tReq.Header.Set("User-Agent", "Walrus/"+s.currentInfo().Version)
	tResp, err := s.httpClient.Do(tReq)
	if err != nil {
		return "", err
	}
	defer tResp.Body.Close()
	if tResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ghcr token HTTP %d", tResp.StatusCode)
	}
	var tok struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(tResp.Body, 128<<10)).Decode(&tok); err != nil {
		return "", err
	}
	if tok.Token == "" {
		return "", errors.New("empty ghcr token")
	}

	// Step 2: HEAD manifest for Docker-Content-Digest
	manURL := fmt.Sprintf("%s/v2/%s/manifests/%s", strings.TrimRight(s.ghcrBase, "/"), repoPath, tag)
	mReq, err := http.NewRequestWithContext(ctx, http.MethodHead, manURL, nil)
	if err != nil {
		return "", err
	}
	mReq.Header.Set("Authorization", "Bearer "+tok.Token)
	mReq.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	}, ", "))
	mReq.Header.Set("User-Agent", "Walrus/"+s.currentInfo().Version)

	mResp, err := s.httpClient.Do(mReq)
	if err != nil {
		return "", err
	}
	defer mResp.Body.Close()
	if mResp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w (HTTP 404)", errGHCRManifestNotFound)
	}
	if mResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ghcr manifest HTTP %d", mResp.StatusCode)
	}
	digest := strings.TrimSpace(mResp.Header.Get("Docker-Content-Digest"))
	if digest == "" {
		return "", errors.New("missing Docker-Content-Digest header")
	}
	return digest, nil
}

func repoBaseName(imageRepo string) string {
	parts := strings.Split(strings.TrimRight(imageRepo, "/"), "/")
	if len(parts) == 0 {
		return "walrus"
	}
	return parts[len(parts)-1]
}
