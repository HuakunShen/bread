package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.github.com"
	maxDownloadBytes  = 512 << 20
)

// Client discovers and installs releases for one repository.
type Client struct {
	Repository     string
	BaseURL        string
	HTTPClient     *http.Client
	CurrentVersion string
	OS             string
	Arch           string
}

// Asset is the subset of a GitHub release asset needed by the updater.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Info describes the latest release and the selected platform archive.
type Info struct {
	Current       string
	Latest        string
	Asset         Asset
	ChecksumAsset Asset
	Available     bool
}

type releaseResponse struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// NewClient returns a client configured for the current process target.
func NewClient(repository, currentVersion string) Client {
	return Client{
		Repository:     repository,
		CurrentVersion: currentVersion,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
	}
}

// Check fetches the latest stable release metadata without downloading it.
func (client Client) Check(ctx context.Context) (Info, error) {
	if strings.TrimSpace(client.Repository) == "" {
		return Info{}, errors.New("release repository is not configured")
	}
	baseURL := strings.TrimRight(client.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", baseURL, client.Repository)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Info{}, fmt.Errorf("create release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "bread-updater/1")

	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return Info{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return Info{}, fmt.Errorf("fetch latest release: GitHub returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var release releaseResponse
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return Info{}, fmt.Errorf("decode latest release: %w", err)
	}
	if release.Draft || release.Prerelease || strings.TrimSpace(release.TagName) == "" {
		return Info{}, errors.New("latest GitHub release is not a stable tagged release")
	}
	asset, err := selectAsset(release.Assets, client.OS, client.Arch)
	if err != nil {
		return Info{}, err
	}
	checksum, err := selectChecksumAsset(release.Assets)
	if err != nil {
		return Info{}, err
	}
	latest := normalizeVersion(release.TagName)
	current := normalizeVersion(client.CurrentVersion)
	available := current == "" || current == "dev" || compareVersions(latest, current) > 0
	return Info{
		Current:       current,
		Latest:        latest,
		Asset:         asset,
		ChecksumAsset: checksum,
		Available:     available,
	}, nil
}

// Upgrade downloads, verifies, extracts, and installs the latest release.
func (client Client) Upgrade(ctx context.Context, output io.Writer) error {
	info, err := client.Check(ctx)
	if err != nil {
		return err
	}
	if !info.Available {
		_, err := fmt.Fprintf(output, "bread is already up to date (%s)\n", info.Latest)
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current executable: %w", err)
	}
	target, err := filepath.EvalSymlinks(executable)
	if err != nil {
		target = executable
	}
	fileInfo, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat current executable: %w", err)
	}
	temporaryDirectory, err := os.MkdirTemp("", "bread-upgrade-")
	if err != nil {
		return fmt.Errorf("create upgrade directory: %w", err)
	}
	keepTemporaryDirectory := false
	defer func() {
		if !keepTemporaryDirectory {
			_ = os.RemoveAll(temporaryDirectory)
		}
	}()

	archivePath := filepath.Join(temporaryDirectory, info.Asset.Name)
	if err := client.download(ctx, info.Asset.BrowserDownloadURL, archivePath); err != nil {
		return err
	}
	checksumPath := filepath.Join(temporaryDirectory, info.ChecksumAsset.Name)
	if err := client.download(ctx, info.ChecksumAsset.BrowserDownloadURL, checksumPath); err != nil {
		return err
	}
	checksums, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksums.txt: %w", err)
	}
	if err := verifyChecksum(checksums, info.Asset.Name, archivePath); err != nil {
		return err
	}
	candidate, err := extractExecutable(archivePath, temporaryDirectory, client.OS == "windows")
	if err != nil {
		return err
	}
	if err := os.Chmod(candidate, fileInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("set executable mode: %w", err)
	}

	if runtime.GOOS == "windows" {
		if err := scheduleWindowsReplacement(candidate, target, temporaryDirectory); err != nil {
			return err
		}
		keepTemporaryDirectory = true
		_, err := fmt.Fprintf(output, "upgrade to %s scheduled; restart bread after the current process exits\n", info.Latest)
		return err
	}
	if err := os.Rename(candidate, target); err != nil {
		return fmt.Errorf("replace current executable: %w", err)
	}
	_, err = fmt.Fprintf(output, "upgraded bread from %s to %s\n", info.Current, info.Latest)
	return err
}

func (client Client) download(ctx context.Context, url, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "bread-updater/1")
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download %q: %w", filepath.Base(destination), err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %q: GitHub returned %s", filepath.Base(destination), response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxDownloadBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write download: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close download: %w", closeErr)
	}
	if written > maxDownloadBytes {
		return fmt.Errorf("download %q exceeds the %d-byte safety limit", filepath.Base(destination), maxDownloadBytes)
	}
	return nil
}

func selectAsset(assets []Asset, goos, goarch string) (Asset, error) {
	osNames := map[string][]string{
		"darwin":  {"darwin", "macos", "osx"},
		"linux":   {"linux"},
		"windows": {"windows"},
	}
	archNames := map[string][]string{
		"amd64": {"amd64", "x86_64", "x86-64"},
		"arm64": {"arm64", "aarch64"},
	}
	allowedOS, ok := osNames[goos]
	if !ok {
		return Asset{}, fmt.Errorf("unsupported update operating system %q", goos)
	}
	allowedArch, ok := archNames[goarch]
	if !ok {
		return Asset{}, fmt.Errorf("unsupported update architecture %q", goarch)
	}
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	var matches []Asset
	for _, asset := range assets {
		lower := strings.ToLower(asset.Name)
		if !strings.HasSuffix(lower, extension) || asset.BrowserDownloadURL == "" {
			continue
		}
		if !containsAny(lower, allowedOS) || !containsAny(lower, allowedArch) {
			continue
		}
		matches = append(matches, asset)
	}
	if len(matches) != 1 {
		return Asset{}, fmt.Errorf("found %d release archives for %s/%s, want exactly one", len(matches), goos, goarch)
	}
	return matches[0], nil
}

func selectChecksumAsset(assets []Asset) (Asset, error) {
	var matches []Asset
	for _, asset := range assets {
		lower := strings.ToLower(asset.Name)
		if strings.Contains(lower, "checksum") && strings.HasSuffix(lower, ".txt") && asset.BrowserDownloadURL != "" {
			matches = append(matches, asset)
		}
	}
	if len(matches) != 1 {
		return Asset{}, fmt.Errorf("found %d checksum assets, want exactly one", len(matches))
	}
	return matches[0], nil
}

func containsAny(value string, choices []string) bool {
	for _, choice := range choices {
		if strings.Contains(value, choice) {
			return true
		}
	}
	return false
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if index := strings.IndexByte(value, '-'); index >= 0 {
		value = value[:index]
	}
	return value
}

func compareVersions(left, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	for index := 0; index < 3; index++ {
		if leftParts[index] > rightParts[index] {
			return 1
		}
		if leftParts[index] < rightParts[index] {
			return -1
		}
	}
	return 0
}

func versionParts(value string) [3]int {
	var result [3]int
	parts := strings.Split(value, ".")
	for index := 0; index < len(parts) && index < len(result); index++ {
		result[index], _ = strconv.Atoi(parts[index])
	}
	return result
}

func verifyChecksum(checksums []byte, archiveName, archivePath string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open downloaded archive: %w", err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		archive.Close()
		return fmt.Errorf("hash downloaded archive: %w", err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close downloaded archive: %w", err)
	}
	want := ""
	scanner := bufio.NewScanner(strings.NewReader(string(checksums)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "*") == archiveName {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read checksum list: %w", err)
	}
	if len(want) != sha256.Size*2 {
		return fmt.Errorf("checksums.txt has no SHA-256 entry for %q", archiveName)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(want, actual) {
		return fmt.Errorf("checksum mismatch for %q", archiveName)
	}
	return nil
}

func extractExecutable(archivePath, destinationDirectory string, windows bool) (string, error) {
	name := "bread"
	if windows {
		name += ".exe"
	}
	destination := filepath.Join(destinationDirectory, name)
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractZip(archivePath, destination, name)
	}
	return extractTarGz(archivePath, destination, name)
}

func extractZip(archivePath, destination, executableName string) (string, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open release zip: %w", err)
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != executableName {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 || !safeArchiveName(entry.Name) {
			return "", errors.New("release archive contains an unsafe executable path")
		}
		reader, err := entry.Open()
		if err != nil {
			return "", fmt.Errorf("open executable in zip: %w", err)
		}
		err = writeLimitedFile(destination, reader, entry.FileInfo().Mode().Perm())
		reader.Close()
		if err != nil {
			return "", err
		}
		return destination, nil
	}
	return "", fmt.Errorf("release zip has no %s executable", executableName)
}

func extractTarGz(archivePath, destination, executableName string) (string, error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open release archive: %w", err)
	}
	defer archive.Close()
	compressed, err := gzip.NewReader(archive)
	if err != nil {
		return "", fmt.Errorf("open gzip release archive: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return "", fmt.Errorf("read release tar: %w", nextErr)
		}
		if filepath.Base(header.Name) != executableName {
			continue
		}
		if header.Typeflag != tar.TypeReg || !safeArchiveName(header.Name) {
			return "", errors.New("release archive contains an unsafe executable entry")
		}
		if err := writeLimitedFile(destination, reader, os.FileMode(header.Mode).Perm()); err != nil {
			return "", err
		}
		return destination, nil
	}
	return "", fmt.Errorf("release archive has no %s executable", executableName)
}

func safeArchiveName(name string) bool {
	normalized := filepath.ToSlash(name)
	if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return false
	}
	cleaned := pathpkg.Clean(normalized)
	return cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func writeLimitedFile(destination string, reader io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o600)
	if err != nil {
		return fmt.Errorf("create extracted executable: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(reader, maxDownloadBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("extract executable: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close extracted executable: %w", closeErr)
	}
	if written > maxDownloadBytes {
		return fmt.Errorf("extracted executable exceeds the %d-byte safety limit", maxDownloadBytes)
	}
	return nil
}

func scheduleWindowsReplacement(candidate, target, temporaryDirectory string) error {
	quote := func(value string) string { return strings.ReplaceAll(value, "\"", "\"\"") }
	logPath := filepath.Join(temporaryDirectory, "upgrade.log")
	scriptPath := filepath.Join(temporaryDirectory, "apply-update.cmd")
	script := fmt.Sprintf("@echo off\r\n"+
		"setlocal EnableExtensions\r\n"+
		"set \"SOURCE=%s\"\r\n"+
		"set \"TARGET=%s\"\r\n"+
		"set \"LOG=%s\"\r\n"+
		"set /a ATTEMPTS=0\r\n"+
		":retry\r\n"+
		"set /a ATTEMPTS+=1\r\n"+
		"move /Y \"%%SOURCE%%\" \"%%TARGET%%\" >nul 2>>\"%%LOG%%\"\r\n"+
		"if not errorlevel 1 goto success\r\n"+
		"if %%ATTEMPTS%% GEQ 120 goto failure\r\n"+
		"timeout /t 1 /nobreak >nul\r\n"+
		"goto retry\r\n"+
		":success\r\n"+
		"del /F /Q \"%%~f0\" >nul 2>&1\r\n"+
		"exit /b 0\r\n"+
		":failure\r\n"+
		"echo bread upgrade failed >>\"%%LOG%%\"\r\n"+
		"exit /b 1\r\n",
		quote(candidate), quote(target), quote(logPath))
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return fmt.Errorf("write Windows upgrade helper: %w", err)
	}
	command := exec.Command("cmd.exe", "/D", "/C", scriptPath)
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Windows upgrade helper: %w", err)
	}
	return nil
}
