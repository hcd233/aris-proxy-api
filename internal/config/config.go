// Package config provides the configuration
package config

import (
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/samber/lo"
	"github.com/spf13/viper"
)

var (
	// Env string 环境
	//	@update 2026-01-31 15:20:42
	Env enum.Env

	// ReadTimeout time Fiber读取超时时间
	//	update 2024-06-22 08:59:40
	ReadTimeout time.Duration

	// WriteTimeout time Fiber写入超时时间
	//	update 2024-06-22 08:59:37
	WriteTimeout time.Duration

	// LogLevel string 日志级别
	//	update 2024-06-22 08:59:29
	LogLevel string

	// LogDirPath string 日志目录路径
	//	update 2024-06-22 08:59:26
	LogDirPath string

	// Oauth2GithubClientID string Github OAuth2 Client ID
	//	update 2024-06-22 08:59:22
	Oauth2GithubClientID string

	// Oauth2GithubClientSecret string Github OAuth2 Client Secret
	//	update 2024-06-22 08:59:17
	Oauth2GithubClientSecret string

	// Oauth2GithubRedirectURL string Github OAuth2 Redirect URL
	//	update 2024-06-22 08:59:07
	Oauth2GithubRedirectURL string

	// Oauth2GoogleClientID string Google OAuth2 Client ID
	Oauth2GoogleClientID string

	// Oauth2GoogleClientSecret string Google OAuth2 Client Secret
	Oauth2GoogleClientSecret string

	// Oauth2GoogleRedirectURL string Google OAuth2 Redirect URL
	Oauth2GoogleRedirectURL string

	// PostgresUser string Postgres用户名
	//	update 2024-06-22 09:00:30
	PostgresUser string

	// PostgresPassword string Postgres密码
	//	update 2024-06-22 09:00:45
	PostgresPassword string

	// PostgresHost string Postgres主机
	//	update 2024-06-22 09:01:02
	PostgresHost string

	// PostgresPort string Postgres端口
	//	update 2024-06-22 09:01:18
	PostgresPort string

	// PostgresDatabase string Postgres数据库
	//	update 2024-06-22 09:01:34
	PostgresDatabase string

	// PostgresSSLMode string Postgres SSL模式
	//	update 2024-06-22 09:01:50
	PostgresSSLMode string

	// RedisHost string Redis主机
	RedisHost string

	// RedisPort string Redis端口
	RedisPort string

	// RedisPassword string Redis密码
	RedisPassword string

	// MinioEndpoint string Minio Endpoint
	MinioEndpoint string

	// MinioTLS bool Minio TLS
	MinioTLS bool

	// MinioRegion string Minio Region
	MinioRegion string

	// MinioBucketName string Minio Bucket Name
	MinioBucketName string

	// MinioAccessID string Minio Access ID
	MinioAccessID string

	// MinioAccessKey string Minio Access Key
	MinioAccessKey string

	// CosRegion string Cos Region
	CosRegion string

	// CosSecretID string Cos Access ID
	CosSecretID string

	// CosSecretKey string Cos Secret Key
	CosSecretKey string

	// CosBucketName string Cos Bucket Name
	CosBucketName string

	// CosAppID string Cos App ID
	CosAppID string

	// JwtAccessTokenExpired time.Duration Access Jwt Token过期时间
	//	update 2024-06-22 11:09:19
	JwtAccessTokenExpired time.Duration

	// JwtAccessTokenSecret string Jwt Access Token密钥
	//	update 2024-06-22 11:15:55
	JwtAccessTokenSecret string

	// JwtRefreshTokenExpired time.Duration Refresh Jwt Token过期时间
	//	update 2024-06-22 11:09:19
	JwtRefreshTokenExpired time.Duration

	// JwtRefreshTokenSecret string Jwt Refresh Token密钥
	//	update 2024-06-22 11:15:55
	JwtRefreshTokenSecret string

	// SQLBatchSize int SQL批量操作大小
	//	@update 2026-03-19 10:00:00
	SQLBatchSize int

	// TrustedProxies []string 可信代理IP列表
	//	@update 2026-04-04 10:00:00
	TrustedProxies []string

	// GuardAllowIPs []string 路由扫描防护白名单 IP 列表
	//	@update 2026-07-01 10:00:00
	GuardAllowIPs []string

	// CLSEndpoint string 腾讯云 CLS Endpoint
	//	@update 2026-04-25 10:00:00
	CLSEndpoint string

	// CLSSecretID string 腾讯云 CLS Secret ID
	//	@update 2026-04-25 10:00:00
	CLSSecretID string

	// CLSSecretKey string 腾讯云 CLS Secret Key
	//	@update 2026-04-25 10:00:00
	CLSSecretKey string

	// CLSTopicID string 腾讯云 CLS 日志主题 ID
	//	@update 2026-04-25 10:00:00
	CLSTopicID string

	// CLSLevel string 腾讯云 CLS 日志级别
	//	@update 2026-04-25 10:00:00
	CLSLevel string

	// CronSessionDeduplicateEnabled bool 是否启用 Session 去重定时任务
	//	@update 2026-05-01 10:00:00
	CronSessionDeduplicateEnabled bool

	// CronSoftDeletePurgeEnabled bool 是否启用软删除清理定时任务
	//	@update 2026-05-01 10:00:00
	CronSoftDeletePurgeEnabled bool

	// CronThinkExtractEnabled bool 是否启用 Think 内容提取定时任务
	//	@update 2026-06-02 10:00:00
	CronThinkExtractEnabled bool

	// UpstreamRetryMaxAttempts int 上游请求重试最大次数（不含首次请求）
	//	@update 2026-06-23 10:00:00
	UpstreamRetryMaxAttempts int

	// UpstreamRetryInitialBackoff time.Duration 上游请求重试初始退避时间
	//	@update 2026-06-23 10:00:00
	UpstreamRetryInitialBackoff time.Duration

	// UpstreamRetryMaxBackoff time.Duration 上游请求重试最大退避时间
	//	@update 2026-06-23 10:00:00
	UpstreamRetryMaxBackoff time.Duration

	// UpstreamRetryJitterFactor float64 上游请求重试退避抖动因子 (0~1)
	//	@update 2026-06-23 10:00:00
	UpstreamRetryJitterFactor float64
)

// PoolGroupConfig 协程池分组配置
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolGroupConfig struct {
	Workers   int
	QueueSize int
}

// PoolConfig 协程池配置
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type PoolConfig struct {
	Store PoolGroupConfig
	Agent PoolGroupConfig
}

// Pool Store 池和 Agent 池的全局配置
var Pool PoolConfig

func init() {
	initEnvironment()
}

func initEnvironment() {
	config := viper.New()
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	config.SetDefault("env", enum.EnvProduction)

	config.SetDefault("read.timeout", 10*time.Second)
	config.SetDefault("write.timeout", 5*time.Minute)

	config.SetDefault("log.level", "info")
	config.SetDefault("log.dir", "./logs")

	config.SetDefault("pool.store.workers", 50)
	config.SetDefault("pool.store.queue_size", 1000)
	config.SetDefault("pool.agent.workers", 10)
	config.SetDefault("pool.agent.queue_size", 100)

	config.SetDefault("sql.batch.size", 500)

	config.SetDefault("postgres.sslmode", "disable")

	config.SetDefault("trusted.proxies", "172.18.0.1")
	config.SetDefault("cron.session.deduplicate.enabled", true)
	config.SetDefault("cron.soft.delete.purge.enabled", true)

	config.SetDefault("upstream.retry.max_attempts", 2)
	config.SetDefault("upstream.retry.initial_backoff", 500*time.Millisecond)
	config.SetDefault("upstream.retry.max_backoff", 2*time.Second)
	config.SetDefault("upstream.retry.jitter_factor", 0.3)

	config.AutomaticEnv()

	Env = config.GetString("env")

	ReadTimeout = config.GetDuration("read.timeout")
	WriteTimeout = config.GetDuration("write.timeout")

	LogLevel = config.GetString("log.level")
	LogDirPath = config.GetString("log.dir")

	Oauth2GithubClientID = config.GetString("oauth2.github.client.id")
	Oauth2GithubClientSecret = config.GetString("oauth2.github.client.secret")
	Oauth2GithubRedirectURL = config.GetString("oauth2.github.redirect.url")

	Oauth2GoogleClientID = config.GetString("oauth2.google.client.id")
	Oauth2GoogleClientSecret = config.GetString("oauth2.google.client.secret")
	Oauth2GoogleRedirectURL = config.GetString("oauth2.google.redirect.url")

	PostgresUser = config.GetString("postgres.user")
	PostgresPassword = config.GetString("postgres.password")
	PostgresHost = config.GetString("postgres.host")
	PostgresPort = config.GetString("postgres.port")
	PostgresDatabase = config.GetString("postgres.database")
	PostgresSSLMode = config.GetString("postgres.sslmode")

	RedisHost = config.GetString("redis.host")
	RedisPort = config.GetString("redis.port")
	RedisPassword = config.GetString("redis.password")

	MinioEndpoint = config.GetString("minio.endpoint")
	MinioTLS = config.GetBool("minio.tls")
	MinioRegion = config.GetString("minio.region")
	MinioBucketName = config.GetString("minio.bucket.name")
	MinioAccessID = config.GetString("minio.access.id")
	MinioAccessKey = config.GetString("minio.access.key")

	CosBucketName = config.GetString("cos.bucket.name")
	CosAppID = config.GetString("cos.app.id")
	CosRegion = config.GetString("cos.region")
	CosSecretID = config.GetString("cos.secret.id")
	CosSecretKey = config.GetString("cos.secret.key")

	JwtAccessTokenExpired = config.GetDuration("jwt.access.token.expired")
	JwtAccessTokenSecret = config.GetString("jwt.access.token.secret")

	JwtRefreshTokenExpired = config.GetDuration("jwt.refresh.token.expired")
	JwtRefreshTokenSecret = config.GetString("jwt.refresh.token.secret")

	SQLBatchSize = config.GetInt("sql.batch.size")

	CLSEndpoint = config.GetString("cls.endpoint")
	CLSSecretID = config.GetString("cls.secret.id")
	CLSSecretKey = config.GetString("cls.secret.key")
	CLSTopicID = config.GetString("cls.topic.id")
	CLSLevel = config.GetString("cls.level")

	CronSessionDeduplicateEnabled = config.GetBool("cron.session.deduplicate.enabled")
	CronSoftDeletePurgeEnabled = config.GetBool("cron.soft.delete.purge.enabled")
	CronThinkExtractEnabled = config.GetBool("cron.think.extract.enabled")

	UpstreamRetryMaxAttempts = config.GetInt("upstream.retry.max_attempts")
	UpstreamRetryInitialBackoff = config.GetDuration("upstream.retry.initial_backoff")
	UpstreamRetryMaxBackoff = config.GetDuration("upstream.retry.max_backoff")
	UpstreamRetryJitterFactor = config.GetFloat64("upstream.retry.jitter_factor")

	Pool = PoolConfig{
		Store: PoolGroupConfig{
			Workers:   config.GetInt("pool.store.workers"),
			QueueSize: config.GetInt("pool.store.queue_size"),
		},
		Agent: PoolGroupConfig{
			Workers:   config.GetInt("pool.agent.workers"),
			QueueSize: config.GetInt("pool.agent.queue_size"),
		},
	}

	if raw := config.GetString("trusted.proxies"); raw != "" {
		TrustedProxies = lo.FilterMap(strings.Split(raw, ","), func(p string, _ int) (string, bool) {
			trimmed := strings.TrimSpace(p)
			return trimmed, trimmed != ""
		})
	}

	if raw := config.GetString("guard.allow_ips"); raw != "" {
		GuardAllowIPs = lo.FilterMap(strings.Split(raw, ","), func(p string, _ int) (string, bool) {
			trimmed := strings.TrimSpace(p)
			return trimmed, trimmed != ""
		})
	}
}
