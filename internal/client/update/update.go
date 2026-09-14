package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Options aris update 运行参数；In/Out/BaseURL/BinaryPath/HTTPClient 为空时使用默认实现
type Options struct {
	Current    string
	In         io.Reader
	Out        io.Writer
	BaseURL    string
	BinaryPath string
	HTTPClient *http.Client
}

// Run 将当前 aris 二进制更新到最新 release；已是最新时只提示不下载
func Run(ctx context.Context, opts Options) error {
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	base := resolveBaseURL(opts.BaseURL)
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: constant.ArisClientUpdateDownloadTimeout}
	}

	latest, err := Latest(ctx, base, hc)
	if err != nil {
		return err
	}
	if IsUpToDate(latest, opts.Current) {
		_, err = fmt.Fprintf(out, constant.ArisClientUpdateUpToDateFormat+"\n", opts.Current)
		return err
	}

	var binary []byte
	title := fmt.Sprintf(constant.ArisClientUpdateDownloadingFormat, latest)
	err = ui.RunWithSpinner(in, out, title, func() error {
		archive, downloadErr := downloadAsset(ctx, base, hc)
		if downloadErr != nil {
			return downloadErr
		}
		if verifyErr := verifyChecksum(ctx, base, hc, archive); verifyErr != nil {
			return verifyErr
		}
		binary, downloadErr = extractBinary(archive)
		return downloadErr
	})
	if err != nil {
		return err
	}

	installed, err := replaceSelf(opts.BinaryPath, binary)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, constant.ArisClientUpdateDoneFormat+"\n", opts.Current, latest, installed)
	return err
}

// Latest 解析最新 release tag：HEAD 资产地址，从第一跳重定向地址中取出版本号
func Latest(ctx context.Context, base string, hc *http.Client) (string, error) {
	asset, err := assetName()
	if err != nil {
		return "", err
	}
	requestCtx, cancel := context.WithTimeout(ctx, constant.ArisClientUpdateCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodHead, base+"/"+asset, http.NoBody)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrBadRequest, err, "create latest release request")
	}
	var redirectURL *url.URL
	client := *hc
	client.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		redirectURL = next.URL
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrProxySend, err, "request latest release")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close

	tag := tagFromURL(redirectURL)
	if _, ok := parseVersion(tag); !ok {
		return "", ierr.New(ierr.ErrValidation, constant.ArisClientUpdateVersionUnknownMessage)
	}
	return tag, nil
}

// assetName 当前平台对应的 release 资产名；不支持的平台返回错误
func assetName() (string, error) {
	switch runtime.GOOS {
	case constant.ArisClientOSDarwin, constant.ArisClientOSLinux:
	default:
		return "", ierr.Newf(ierr.ErrValidation, constant.ArisClientUpdateUnsupportedPlatformFormat, runtime.GOOS, runtime.GOARCH)
	}
	switch runtime.GOARCH {
	case constant.ArisClientArchAMD64, constant.ArisClientArchARM64:
	default:
		return "", ierr.Newf(ierr.ErrValidation, constant.ArisClientUpdateUnsupportedPlatformFormat, runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf(constant.ArisClientReleaseAssetFormat, runtime.GOOS, runtime.GOARCH), nil
}

// tagFromURL 从 releases/download/<tag>/<asset> 形式的地址中取出版本 tag
func tagFromURL(target *url.URL) string {
	if target == nil {
		return ""
	}
	segments := strings.Split(target.Path, "/")
	index := slices.Index(segments, constant.ArisClientReleaseDownloadSegment)
	if index < 0 || index+1 >= len(segments) {
		return ""
	}
	return segments[index+1]
}

// downloadAsset 下载当前平台的 release 归档
func downloadAsset(ctx context.Context, base string, hc *http.Client) ([]byte, error) {
	asset, err := assetName()
	if err != nil {
		return nil, err
	}
	return getBody(ctx, hc, base+"/"+asset, constant.ArisClientUpdateMaxArchiveBytes)
}

// verifyChecksum 下载 .sha256 并比对归档摘要
func verifyChecksum(ctx context.Context, base string, hc *http.Client, archive []byte) error {
	asset, err := assetName()
	if err != nil {
		return err
	}
	checksumURL := base + "/" + asset + constant.ArisClientReleaseChecksumSuffix
	data, err := getBody(ctx, hc, checksumURL, constant.ArisClientUpdateMaxArchiveBytes)
	if err != nil {
		return err
	}
	expected := parseChecksum(string(data))
	if expected == "" {
		return ierr.New(ierr.ErrValidation, constant.ArisClientUpdateChecksumMessage)
	}
	sum := sha256.Sum256(archive)
	if !strings.EqualFold(expected, hex.EncodeToString(sum[:])) {
		return ierr.New(ierr.ErrValidation, constant.ArisClientUpdateChecksumMessage)
	}
	return nil
}

// parseChecksum 从 sha256 文件内容中取第一段合法摘要
func parseChecksum(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		digest, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		if len(digest) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(digest); err == nil {
			return digest
		}
	}
	return ""
}

// getBody 发起 GET 并读取响应体（读取上限 maxBytes）
func getBody(ctx context.Context, hc *http.Client, target string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrBadRequest, err, "create release asset request")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxySend, err, "download release asset")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, ierr.Newf(ierr.ErrProxySend, "release asset rejected with status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxySend, err, "read release asset")
	}
	return data, nil
}

// extractBinary 从 tar.gz 归档中取出 aris 二进制
func extractBinary(archive []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "open release archive")
	}
	defer func() { _ = gzipReader.Close() }() //nolint:errcheck // best-effort close

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ierr.Wrap(ierr.ErrValidation, err, "read release archive")
		}
		if header.Name != constant.ArisClientBinaryFileName {
			continue
		}
		binary, err := io.ReadAll(io.LimitReader(tarReader, constant.ArisClientUpdateMaxArchiveBytes))
		if err != nil {
			return nil, ierr.Wrap(ierr.ErrValidation, err, "extract aris binary")
		}
		return binary, nil
	}
	return nil, ierr.New(ierr.ErrValidation, constant.ArisClientUpdateArchiveMemberMessage)
}

// replaceSelf 用 binary 原子替换目标二进制；targetPath 为空时解析当前可执行文件
func replaceSelf(targetPath string, binary []byte) (string, error) {
	if targetPath == "" {
		resolved, err := trace.ExecutablePath()
		if err != nil {
			return "", err
		}
		targetPath = resolved
	}
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, constant.ArisClientUpdateTempPattern)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "create update temporary file")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() //nolint:errcheck // best-effort cleanup

	if err := tmp.Chmod(constant.ArisClientUpdateBinaryMode); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return "", ierr.Wrap(ierr.ErrInternal, err, "secure update temporary file")
	}
	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return "", ierr.Wrap(ierr.ErrInternal, err, "write update temporary file")
	}
	if err := tmp.Close(); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "close update temporary file")
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "replace aris binary")
	}
	return targetPath, nil
}

// resolveBaseURL 解析更新源：显式参数 > ARIS_UPDATE_BASE_URL > 内置默认
func resolveBaseURL(base string) string {
	if base != "" {
		return strings.TrimRight(base, "/")
	}
	if fromEnv := strings.TrimSpace(os.Getenv(constant.ArisClientUpdateBaseURLEnv)); fromEnv != "" {
		return strings.TrimRight(fromEnv, "/")
	}
	return constant.ArisClientReleaseBaseURL
}
