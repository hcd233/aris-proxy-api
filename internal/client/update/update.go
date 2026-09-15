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
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/client/executable"
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
	base, err := resolveBaseURL(opts.BaseURL)
	if err != nil {
		return err
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = newHTTPClient(constant.ArisClientUpdateDownloadTimeout)
	}

	// 显式更新命令容忍慢网络：探测预算与下载预算同量级，使用中检查才用 1.5s
	probeCtx, cancelProbe := context.WithTimeout(ctx, constant.ArisClientUpdateProbeTimeout)
	defer cancelProbe()

	latest, err := Latest(probeCtx, base, hc)
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

// Latest 解析最新 release tag：HEAD 资产地址，从第一跳重定向地址中取出版本号。
// 超时由调用方通过 ctx 控制（使用中检查 1.5s，aris update 30s）。
func Latest(ctx context.Context, base string, hc *http.Client) (string, error) {
	asset, err := assetName()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, base+"/"+asset, http.NoBody)
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
	expected, err := parseChecksum(string(data), asset)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	if !strings.EqualFold(expected, hex.EncodeToString(sum[:])) {
		return ierr.New(ierr.ErrValidation, constant.ArisClientUpdateChecksumMessage)
	}
	return nil
}

// parseChecksum 解析 sha256 文件：必须恰好一行有效摘要，摘要是 64 位 hex，
// 带文件名（sha256sum 两列格式）时必须与当前资产名一致。口径与 install_aris_client.sh.tmpl 一致。
func parseChecksum(content, asset string) (string, error) {
	invalid := func() (string, error) {
		return "", ierr.New(ierr.ErrValidation, constant.ArisClientUpdateChecksumFormatMessage)
	}
	digest := ""
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if digest != "" ||
			len(fields) > 2 ||
			len(fields[0]) != sha256.Size*2 ||
			len(fields) == 2 && strings.TrimPrefix(fields[1], "*") != asset {
			return invalid()
		}
		if _, decodeErr := hex.DecodeString(fields[0]); decodeErr != nil {
			return invalid()
		}
		digest = fields[0]
	}
	if digest == "" {
		return invalid()
	}
	return digest, nil
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
		return nil, ierr.Newf(ierr.ErrProxySend, constant.ArisClientUpdateAssetStatusFormat, target, resp.StatusCode)
	}
	// 多读 1 字节用于区分「恰好等于上限」与「超过上限被截断」：
	// 截断若被静默接受，最终只会报成校验失败，无法定位是产物损坏还是下载被掐断。
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxySend, err, "read release asset")
	}
	if int64(len(data)) > maxBytes {
		return nil, ierr.Newf(ierr.ErrProxySend, constant.ArisClientUpdateArchiveTooLargeFormat, maxBytes)
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
		resolved, err := executable.Path()
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
	// 先落盘再 rename：否则断电后 rename 可能指向尚未回写的空/截断文件，
	// 用户唯一的 aris 二进制随之损坏且无回滚点（ext4 有 auto_da_alloc 缓解，APFS 不保证）。
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return "", ierr.Wrap(ierr.ErrInternal, err, "sync update temporary file")
	}
	if err := tmp.Close(); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "close update temporary file")
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "replace aris binary")
	}
	// best-effort 落盘目录项：rename 的持久化需要父目录 fsync，
	// 部分文件系统（darwin 的若干卷）不支持目录 fsync，失败不影响安装结果。
	if handle, openErr := os.Open(dir); openErr == nil { //nolint:gosec // dir 来自目标二进制所在目录
		_ = handle.Sync()  //nolint:errcheck // best-effort durability
		_ = handle.Close() //nolint:errcheck // best-effort close
	}
	return targetPath, nil
}

// resolveBaseURL 解析更新源并校验：显式参数 > ARIS_UPDATE_BASE_URL > 内置默认
func resolveBaseURL(base string) (string, error) {
	if base == "" {
		base = strings.TrimSpace(os.Getenv(constant.ArisClientUpdateBaseURLEnv))
	}
	if base == "" {
		base = constant.ArisClientReleaseBaseURL
	}
	resolved := strings.TrimRight(base, "/")
	if err := validateBaseURL(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// validateBaseURL 校验更新源：必须 https；http 只允许 loopback（本地镜像与测试），
// 否则等于用明文下发可执行文件。
func validateBaseURL(base string) error {
	parsed, err := url.Parse(base)
	if err != nil {
		return ierr.Wrap(ierr.ErrValidation, err, "parse update base url")
	}
	if parsed.Host == "" {
		return ierr.Newf(ierr.ErrValidation, constant.ArisClientUpdateInsecureSourceFormat, base)
	}
	if parsed.Scheme == constant.ArisClientSchemeHTTPS {
		return nil
	}
	ip := net.ParseIP(parsed.Hostname())
	if parsed.Hostname() == constant.ArisClientLoopbackHost || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return ierr.Newf(ierr.ErrValidation, constant.ArisClientUpdateInsecureSourceFormat, base)
}

// newHTTPClient 构造自更新 HTTP client：限制重定向跳数，且每一跳都必须通过更新源校验，
// 挡掉 https→http 降级与跨源明文跳转（归档与校验文件同源下载，静默跟随重定向会一起被换掉）。
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= constant.ArisClientUpdateMaxRedirects {
				return ierr.New(ierr.ErrValidation, constant.ArisClientUpdateTooManyRedirectsMessage)
			}
			return validateBaseURL(req.URL.Scheme + "://" + req.URL.Host)
		},
	}
}
