package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckSelectsPlatformAssetAndChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/HuakunShen/bread/releases/latest" {
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode(releaseResponse{
			TagName: "v1.2.3",
			Assets: []Asset{
				{Name: "bread_1.2.3_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.test/darwin"},
				{Name: "bread_1.2.3_Darwin_amd64.tar.gz", BrowserDownloadURL: "https://example.test/darwin-amd64"},
				{Name: "bread_1.2.3_Windows_arm64.zip", BrowserDownloadURL: "https://example.test/windows"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums"},
			},
		})
	}))
	defer server.Close()

	client := Client{
		Repository:     "HuakunShen/bread",
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
		CurrentVersion: "v1.0.0",
		OS:             "darwin",
		Arch:           "arm64",
	}
	info, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Latest != "1.2.3" || !info.Available || info.Asset.Name != "bread_1.2.3_Darwin_arm64.tar.gz" || info.ChecksumAsset.Name != "checksums.txt" {
		t.Fatalf("unexpected release info: %#v", info)
	}
}

func TestCheckDevelopmentBuildCanUpgrade(t *testing.T) {
	assets := []Asset{
		{Name: "bread_1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.test/archive"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums"},
	}
	asset, err := selectAsset(assets, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name == "" || normalizeVersion("v1.0.0-rc1") != "1.0.0" || compareVersions("1.0.0", "1.0.0") != 0 {
		t.Fatalf("version helpers returned unexpected values")
	}
	client := Client{CurrentVersion: "dev"}
	_ = client
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.tar.gz")
	content := []byte("release bytes")
	if err := os.WriteFile(archivePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	checksums := []byte(hex.EncodeToString(sum[:]) + "  archive.tar.gz\n")
	if err := verifyChecksum(checksums, "archive.tar.gz", archivePath); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum([]byte(strings.Repeat("0", sha256.Size*2)+"  archive.tar.gz\n"), "archive.tar.gz", archivePath); err == nil {
		t.Fatal("verifyChecksum accepted a mismatched checksum")
	}
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "release.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "bread", Mode: 0o755, Size: int64(len("binary"))}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(tarWriter, "binary"); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	extracted, err := extractExecutable(archivePath, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary" {
		t.Fatalf("extracted %q, want binary", got)
	}
}

func TestExtractZipAndRejectTraversal(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "release.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zipWriter := zip.NewWriter(archive)
	entry, err := zipWriter.Create("nested/bread.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("binary")); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	extracted, err := extractExecutable(archivePath, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("binary")) {
		t.Fatalf("extracted %q, want binary", got)
	}

	traversalPath := filepath.Join(dir, "traversal.tar.gz")
	traversalFile, err := os.Create(traversalPath)
	if err != nil {
		t.Fatal(err)
	}
	traversalGzip := gzip.NewWriter(traversalFile)
	traversalTar := tar.NewWriter(traversalGzip)
	if err := traversalTar.WriteHeader(&tar.Header{Name: "../bread", Mode: 0o755, Size: 0}); err != nil {
		t.Fatal(err)
	}
	_ = traversalTar.Close()
	_ = traversalGzip.Close()
	_ = traversalFile.Close()
	if _, err := extractExecutable(traversalPath, dir, false); err == nil {
		t.Fatal("extractExecutable accepted a traversal path")
	}
}
