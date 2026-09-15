package constant

import "time"

// aris 客户端自更新（aris update）与使用中更新检查常量
const (
	// ArisClientReleaseBaseURL release 资产源；与 install_aris_client.sh.tmpl 的 gh_base 同源
	ArisClientReleaseBaseURL = "https://github.com/hcd233/aris-proxy-api/releases/latest/download"

	// ArisClientReleaseAssetFormat release 资产名模板（os, arch）
	ArisClientReleaseAssetFormat = "aris-%s-%s.tar.gz"
	// ArisClientReleaseChecksumSuffix 资产校验文件后缀
	ArisClientReleaseChecksumSuffix = ".sha256"
	// ArisClientReleaseDownloadSegment 重定向地址中携带 tag 的路径段：releases/download/<tag>/<asset>
	ArisClientReleaseDownloadSegment = "download"

	// ArisClientUpdateBaseURLEnv 覆盖更新源（镜像/测试用）
	ArisClientUpdateBaseURLEnv = "ARIS_UPDATE_BASE_URL"
	// ArisClientNoUpdateCheckEnv 置为真值即关闭使用中更新检查
	ArisClientNoUpdateCheckEnv = "ARIS_NO_UPDATE_CHECK"
	// ArisClientEnvValueTrueList 环境变量视为真值的取值（大小写不敏感）
	ArisClientEnvValueTrueList = "1,true,yes,on"
	// ArisClientEnvValueSeparator 真值列表分隔符
	ArisClientEnvValueSeparator = ","

	// ArisClientUpdateCheckTimeout 使用中检查（HEAD 取 tag）超时
	ArisClientUpdateCheckTimeout = 1500 * time.Millisecond
	// ArisClientUpdateProbeTimeout aris update 解析最新 tag 的超时（显式命令容忍慢网络）
	ArisClientUpdateProbeTimeout = 30 * time.Second
	// ArisClientUpdateNoticeWait 命令结束后等待提示的最长时间
	ArisClientUpdateNoticeWait = 400 * time.Millisecond
	// ArisClientUpdateDownloadTimeout 下载、校验与替换的整体超时
	ArisClientUpdateDownloadTimeout = 5 * time.Minute
	// ArisClientUpdateMaxRedirects 更新源重定向跳数上限（默认 10 跳）
	ArisClientUpdateMaxRedirects = 10
	// ArisClientUpdateMaxArchiveBytes 归档下载与解包的体积上限
	ArisClientUpdateMaxArchiveBytes = 64 << 20
	// ArisClientLoopbackHost 允许 http 更新的本地主机名（mirror/测试）
	ArisClientLoopbackHost = "localhost"

	// ArisClientUpdateTempPattern 自替换临时文件模板（与目标二进制同目录）
	ArisClientUpdateTempPattern = ".aris-update-*"
	// ArisClientUpdateBinaryMode 安装后的二进制权限
	ArisClientUpdateBinaryMode = 0o700

	// ArisClientDevVersion 本地构建的版本号（release 构建经 -ldflags -X main.version 覆盖）
	ArisClientDevVersion = "dev"
	// ArisClientVersionPrefixV 版本 tag 的 v 前缀
	ArisClientVersionPrefixV = "v"

	// ArisClientCommandTrace hook 高频命令，不参与使用中更新检查
	ArisClientCommandTrace = "trace"
	// ArisClientCommandUpdate 自更新命令，不参与使用中更新检查
	ArisClientCommandUpdate = "update"
	// ArisClientCommandVersion 版本命令，不参与使用中更新检查
	ArisClientCommandVersion = "version"
)

// aris 客户端更新提示与 aris update 输出文案
const (
	ArisClientUpdateAvailableFormat           = "! A new version %s is available (current %s) — run `aris update`"
	ArisClientUpdateUpToDateFormat            = "aris is up to date (%s)"
	ArisClientUpdateDoneFormat                = "Updated aris %s → %s (%s)"
	ArisClientUpdateDownloadingFormat         = "Downloading aris %s..."
	ArisClientUpdateVersionUnknownMessage     = "Failed to determine the latest aris version."
	ArisClientUpdateChecksumMessage           = "Checksum verification failed."
	ArisClientUpdateChecksumFormatMessage     = "Invalid checksum file."
	ArisClientUpdateArchiveMemberMessage      = "Downloaded archive does not contain the aris binary."
	ArisClientUpdateUnsupportedPlatformFormat = "Unsupported platform: %s/%s"
	ArisClientUpdateInsecureSourceFormat      = "Refusing insecure update source: %s"
	ArisClientUpdateTooManyRedirectsMessage   = "Too many redirects while fetching the release asset."
	ArisClientUpdateAssetStatusFormat         = "Release asset %s returned status %d."
	ArisClientUpdateArchiveTooLargeFormat     = "Downloaded release asset exceeds the %d byte limit."
)
