// Types matching backend DTOs in internal/dto/

// ─── Common ────────────────────────────────────────────────────────────────────

export interface ApiError {
  code: number;
  message: string;
}

export interface CommonRsp {
  error: ApiError | null;
}

export interface PageInfo {
  page: number;
  pageSize: number;
  total: number;
}

// ─── Permission ─────────────────────────────────────────────────────────────────

export type Permission = "pending" | "demo" | "user" | "admin";

// ─── Demo Account ──────────────────────────────────────────────────────────────

export type DemoModule =
  "dashboard" | "sessions" | "audit" | "upstream" | "trigger" | "monitor" | "cron" | "cron_audit";

export interface DemoStatusRsp extends CommonRsp {
  loginEnabled: boolean;
  demoUserExists: boolean;
}

export interface DemoLoginRsp extends CommonRsp {
  accessToken?: string;
  refreshToken?: string;
}

export interface DemoSession {
  id: number;
  summary?: string;
  score?: number;
  messageCount: number;
  toolCount: number;
  createdAt?: string;
  modelIds?: string[];
}

export interface ListDemoSessionsRsp extends CommonRsp {
  sessions?: DemoSession[];
  pageInfo?: PageInfo;
}

export interface AddDemoSessionsReqBody {
  sessionIds: number[];
}

export interface DemoConfig {
  loginEnabled: boolean;
  modules: DemoModule[];
  updatedAt?: string;
}

export interface GetDemoConfigRsp extends CommonRsp {
  config?: DemoConfig;
}

export interface UpdateDemoConfigReqBody {
  config: {
    loginEnabled?: boolean;
    modules?: DemoModule[];
  };
}

// ─── Auth / OAuth2 ─────────────────────────────────────────────────────────────

export type OAuth2Provider = "github" | "google";

export interface LoginRsp extends CommonRsp {
  redirectURL?: string;
}

export interface CallbackReqBody {
  platform: OAuth2Provider;
  code: string;
  state: string;
}

export interface CallbackRsp extends CommonRsp {
  accessToken?: string;
  refreshToken?: string;
}

export interface RefreshTokenReqBody {
  refreshToken: string;
}

export interface RefreshTokenRsp extends CommonRsp {
  accessToken?: string;
  refreshToken?: string;
}

// ─── User ──────────────────────────────────────────────────────────────────────

export interface User {
  name?: string;
  email?: string;
  avatar?: string;
}

export interface DetailedUser {
  id: number;
  createdAt?: string;
  lastLogin?: string;
  permission?: Permission;
  name?: string;
  email?: string;
  avatar?: string;
}

export interface GetCurUserRsp extends CommonRsp {
  user?: DetailedUser;
}

// ─── User Management ────────────────────────────────────────────────────────────

export interface UserItem {
  id: number;
  name: string;
  email: string;
  avatar?: string;
  permission: Permission;
  createdAt?: string;
  lastLogin?: string;
}

export interface ListUsersRsp extends CommonRsp {
  items?: UserItem[];
  pageInfo?: PageInfo;
}

export interface UpdateUserReqBody {
  user: User;
}

export interface DemoteUserReq {
  id: number;
}

export interface DeleteUserReq {
  id: number;
}

// ─── Session ───────────────────────────────────────────────────────────────────

export interface SessionSummary {
  /** 来源 Agent Harness（后端由 UA 现算；未识别为 "Other"） */
  agentHarness?: string;
  id: number;
  createdAt: string;
  updatedAt: string;
  summary: string;
  score?: number;
  messageCount: number;
  toolCount: number;
  metadata?: Record<string, string>;
  modelIds?: string[];
}

export interface SessionDetail {
  id: number;
  apiKeyName: string;
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
  score?: number;
  scoredAt?: string;
  messages: MessageItem[];
  tools: ToolItem[];
  shareID?: string;
}

export interface UnifiedToolCall {
  id?: string;
  name: string;
  arguments: string; // JSON string
}

export interface UnifiedMessage {
  role: string;
  content?: string | Array<Record<string, unknown>>;
  name?: string;
  reasoning_content?: string;
  refusal?: string;
  tool_call_id?: string;
  tool_calls?: UnifiedToolCall[];
  raw_content?: string;
}

export interface MessageItem {
  id: number;
  modelId: string;
  message: UnifiedMessage;
  createdAt: string;
}

export interface UnifiedTool {
  name: string;
  description: string;
  parameters: Record<string, unknown>; // JSON Schema
}

export interface ToolItem {
  id: number;
  tool: UnifiedTool;
  createdAt: string;
}

export interface ListSessionsRsp extends CommonRsp {
  sessions?: SessionSummary[];
  pageInfo?: PageInfo;
}

export interface GetSessionRsp extends CommonRsp {
  session?: SessionDetail;
}

// ─── Session Detail Perf (新增：metadata + 分页接口) ────────────────────────────

export interface SessionMetadata {
  id: number;
  apiKeyName: string;
  /** 来源 Agent Harness（后端由 UA 现算；未识别为 "Other"） */
  agentHarness?: string;
  /** 会话创建时的原始 User-Agent（仅详情返回） */
  clientUA?: string;
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
  score?: number;
  scoredAt?: string;
  messageCount: number;
  toolCount: number;
  shareID?: string;
}

export interface GetSessionMetadataRsp extends CommonRsp {
  session?: SessionMetadata;
}

export interface ListSessionMessagesRsp extends CommonRsp {
  messages?: MessageItem[];
  pageInfo?: PageInfo;
}

export interface ListSessionToolsRsp extends CommonRsp {
  tools?: ToolItem[];
  pageInfo?: PageInfo;
}

// ─── Session Score ───────────────────────────────────────────────────────────

export interface ScoreSessionReqBody {
  sessionId: number;
  score: number;
}

export interface ScoreSessionRsp extends CommonRsp {
  sessionId: number;
  score: number;
  scoredAt?: string;
}

// ─── Session Share ────────────────────────────────────────────────────────────

export interface CreateShareReqBody {
  sessionId: number;
  expiresIn?: string;
  expiresAt?: number;
}

export interface CreateShareRsp extends CommonRsp {
  shareId?: string;
  expiresAt?: string;
}

// ─── Session Delete ─────────────────────────────────────────────────────────

export interface DeleteSessionFailed {
  id: number;
  error: string;
}

export interface DeleteSessionRsp extends CommonRsp {
  deletedCount?: number;
  failures?: DeleteSessionFailed[];
}

// ─── Session Batch Delete ─────────────────────────────────────────────────────

// ─── Share 分页接口（公开，与 session detail 优化模式对齐） ───────────────────

export interface ShareSessionMetadata {
  id: number;
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
  score?: number;
  messageCount: number;
  toolCount: number;
}

export interface GetShareMetadataRsp extends CommonRsp {
  session?: ShareSessionMetadata;
}

export interface ListShareMessagesRsp extends CommonRsp {
  messages?: MessageItem[];
  pageInfo?: PageInfo;
}

export interface ListShareToolsRsp extends CommonRsp {
  tools?: ToolItem[];
  pageInfo?: PageInfo;
}

export interface ShareItem {
  shareId: string;
  sessionId: number;
  createdAt: string;
  expiresAt: string;
}

export interface ListSharesRsp extends CommonRsp {
  shares?: ShareItem[];
  pageInfo?: PageInfo;
}

// ─── API Key ───────────────────────────────────────────────────────────────────

export interface APIKeyUser {
  id: number;
  name: string;
  avatar: string;
}

export interface APIKeyItem {
  id: number;
  name: string;
  key: string; // masked
  user?: APIKeyUser;
  createdAt: string;
}

export interface APIKeyDetail {
  id: number;
  name: string;
  key: string; // full key, only on creation
  createdAt: string;
}

export interface CreateAPIKeyReqBody {
  name: string;
}

export interface CreateAPIKeyRsp extends CommonRsp {
  key?: APIKeyDetail;
}

export interface ListAPIKeysRsp extends CommonRsp {
  keys?: APIKeyItem[];
  pageInfo?: PageInfo;
}

// ─── Endpoint ──────────────────────────────────────────────────────────────────

/** 端点认证类型：API Key 直连 / Claude 订阅 OAuth / Codex 订阅 OAuth */
export type EndpointAuthType = "api_key" | "oauth_anthropic" | "oauth_openai";

/** 端点可见性：私有（默认，仅归属者与 admin）/ 公开（全体登录用户只读可见、可调用） */
export type EndpointVisibility = "private" | "public";

/** 订阅账号提供方（发起授权时选择） */
export type OAuthProviderChoice = "anthropic" | "openai";

export interface OAuthQuotaWindowDTO {
  key: string;
  usedPercent: number;
  resetsAt: string;
  windowMinutes: number;
}

export interface OAuthQuotaSnapshotDTO {
  windows: OAuthQuotaWindowDTO[];
  planType?: string;
  capturedAt: string;
}

/** 订阅端点凭据投影（仅展示字段；token 永不下发） */
export interface UpstreamOAuthView {
  accountEmail?: string;
  planType?: string;
  /** active / reauth_required */
  status: string;
  lastError?: string;
  quota?: OAuthQuotaSnapshotDTO;
  quotaUpdatedAt?: string;
}

export interface AuthorizeOAuthReqBody {
  provider: OAuthProviderChoice;
}

export interface AuthorizeOAuthRspBody {
  authorizeURL: string;
  state: string;
}

export interface ExchangeOAuthReqBody {
  ownerUserID?: number;
  name: string;
  provider: OAuthProviderChoice;
  code: string;
  state?: string;
}

export interface ExchangeOAuthRspBody {
  endpointID: number;
}

export interface ReauthOAuthReqBody {
  code: string;
  state?: string;
}

export interface OAuthQuotaRspBody {
  status: string;
  planType?: string;
  lastError?: string;
  quota?: OAuthQuotaSnapshotDTO;
  quotaUpdatedAt?: string;
}

export interface CreateEndpointReqBody {
  ownerUserID?: number;
  name: string;
  visibility?: EndpointVisibility;
  openaiBaseURL?: string;
  anthropicBaseURL?: string;
  apiKey: string;
  supportOpenAIChatCompletion?: boolean;
  supportOpenAIResponse?: boolean;
  supportAnthropicMessage?: boolean;
  supportOpenAIDecision?: boolean;
}

export interface UpdateEndpointReqBody {
  name?: string;
  visibility?: EndpointVisibility;
  openaiBaseURL?: string;
  anthropicBaseURL?: string;
  apiKey?: string;
  supportOpenAIChatCompletion?: boolean;
  supportOpenAIResponse?: boolean;
  supportAnthropicMessage?: boolean;
  supportOpenAIDecision?: boolean;
}

// ─── Upstream (endpoint 分组视图) ─────────────────────────────────────────────

export interface UpstreamUser {
  id: number;
  name: string;
  avatar: string;
}

export interface UpstreamEndpointItem {
  id: number;
  user?: UpstreamUser;
  name: string;
  openaiBaseURL: string;
  anthropicBaseURL: string;
  maskedAPIKey: string;
  /** 认证类型（api_key / oauth_anthropic / oauth_openai） */
  authType: EndpointAuthType;
  /** 可见性（private / public）；public 对普通用户只读 */
  visibility: EndpointVisibility;
  /** 订阅端点凭据投影；仅 OAuth 端点非空 */
  oauth?: UpstreamOAuthView;
  supportOpenAIChatCompletion: boolean;
  supportOpenAIResponse: boolean;
  supportAnthropicMessage: boolean;
  supportOpenAIDecision: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface UpstreamModelItem {
  id: number;
  user?: UpstreamUser;
  alias: string;
  modelId: string;
  upstreamModel: string;
  enabled: boolean;
  contextLength: number;
  maxOutputTokens: number;
  capabilities: ModelCapability[];
  pricing?: PricingDTO;
  /** 配置缺失项：pricing(未计价)/spec(未填规格) */
  configMissing?: ConfigMissing[];
  /** 调度优先级（数字小=优先级高） */
  priority: number;
  /** 同优先级加权随机权重 */
  weight: number;
  createdAt: string;
  updatedAt: string;
}

export interface UpstreamGroupItem {
  endpoint: UpstreamEndpointItem;
  models: UpstreamModelItem[];
  modelCount: number;
  /** 截断前口径；未截断时等于 modelCount */
  totalModelCount?: number;
  truncated?: boolean;
}

export interface ListUpstreamRsp extends CommonRsp {
  groups?: UpstreamGroupItem[];
  pageInfo?: PageInfo;
  modelTotal?: number;
}

// ─── Model ─────────────────────────────────────────────────────────────────────

export type ModelCapability = "text" | "image" | "pdf" | "video" | "audio";

/** 模型配置缺失项（enum.ConfigMissing）：pricing=未计价，spec=未填规格 */
export type ConfigMissing = "pricing" | "spec";

// ─── Pricing（模型定价，wire 展示单位：货币单位/1M tokens） ──────────────────

export type PricingCurrency = "" | "USD";

/** 时段窗口：[start,end) 半开区间，end<start 表示跨午夜；days 空=每天（1=周一…7=周日） */
export interface TimeWindowDTO {
  days?: number[];
  start: string;
  end: string;
  timezone?: string;
}

/** 定价规则（数组顺序=匹配优先级；须恰好一条无条件默认规则） */
export interface PricingRuleDTO {
  time_windows?: TimeWindowDTO[];
  /** prompt token 下界（含） */
  context_min?: number;
  /** 上界（不含），0/缺省=无上限 */
  context_max?: number;
  input_price: number;
  output_price: number;
  cache_creation_price: number;
  /** 1h 缓存创建单价（可选；0/缺省=回落 5m 档 cache_creation_price） */
  cache_creation_1h_price?: number;
  cache_read_price: number;
}

export interface PricingDTO {
  currency?: PricingCurrency;
  rules?: PricingRuleDTO[];
}

/** 模型规格导入响应（found=false 表示未命中/上游不可达，其余字段零值）。
 * 仅用于填充表单，永不自动改价；pricing 为完整定价（含上下文区间规则），币种恒 USD。 */
export interface ModelSpecPrefillRsp extends CommonRsp {
  found: boolean;
  contextLength?: number;
  maxOutputTokens?: number;
  capabilities?: ModelCapability[];
  pricing?: PricingDTO;
}

/** 模型成本排行行（展示单位；行 = 模型 × 币种） */
export interface ModelCostItem {
  modelId: string;
  currency: string;
  inputCost: number;
  outputCost: number;
  cacheCreationCost: number;
  cacheReadCost: number;
  totalCost: number;
}

export interface ModelCostRsp extends CommonRsp {
  data?: ModelCostItem[];
}

export interface CreateModelReqBody {
  alias: string;
  modelId?: string;
  upstreamModel: string;
  endpointID: number;
  contextLength?: number;
  maxOutputTokens?: number;
  capabilities?: ModelCapability[];
  /** 定价（缺省=未计价） */
  pricing?: PricingDTO;
  /** 调度优先级（数字小=优先级高，缺省 0） */
  priority?: number;
  /** 同优先级加权随机权重（缺省 1） */
  weight?: number;
}

export interface UpdateModelReqBody {
  alias?: string;
  modelId?: string;
  /** modelId 变化时同步更新历史记录（audit/session/message） */
  syncHistory?: boolean;
  upstreamModel?: string;
  endpointID?: number;
  enabled?: boolean;
  contextLength?: number;
  maxOutputTokens?: number;
  capabilities?: ModelCapability[];
  /** 定价（缺省=不修改；currency 与 rules 均置空=清空为未计价） */
  pricing?: PricingDTO;
  /** 调度优先级（数字小=优先级高） */
  priority?: number;
  /** 同优先级加权随机权重 */
  weight?: number;
}

/** 更新模型响应：历史同步的各表影响行数（未同步时全 0） */
export interface ModelUpdateRsp {
  auditCount: number;
  sessionCount: number;
  messageCount: number;
}

// ─── Model flat list（平铺视图，GET /api/web/v1/model/list） ───────────────────

/** 平铺列表中的所属端点（仅展示所需最小字段，不含 baseURL / apiKey） */
export interface ModelListEndpoint {
  id: number;
  name: string;
  /** 可见性（private / public）；public 对普通用户只读 */
  visibility: EndpointVisibility;
}

export interface ModelListItem {
  id: number;
  user?: UpstreamUser;
  endpoint?: ModelListEndpoint;
  alias: string;
  modelId: string;
  upstreamModel: string;
  enabled: boolean;
  contextLength: number;
  maxOutputTokens: number;
  capabilities: ModelCapability[];
  pricing?: PricingDTO;
  /** 配置缺失项：pricing(未计价)/spec(未填规格) */
  configMissing?: ConfigMissing[];
  /** 调度优先级（数字小=优先级高） */
  priority: number;
  /** 同优先级加权随机权重 */
  weight: number;
  createdAt: string;
  updatedAt: string;
}

export interface ListModelsPageRsp extends CommonRsp {
  items?: ModelListItem[];
  pageInfo?: PageInfo;
}

/**
 * 后端排序白名单（constant.ModelListSortFields）。
 * 前端不得传白名单外的值——后端对非法值静默回退默认列（不报错），
 * 传错不会失败但排序结果会出乎意料。
 */
export type ModelListSortField =
  "alias" | "context_length" | "max_output_tokens" | "created_at" | "endpoint_id" | "enabled";

// ─── Audit ─────────────────────────────────────────────────────────────────────

export interface AuditLogItem {
  id: number;
  createdAt: string;
  modelId: string;
  upstreamProtocol: string;
  apiProtocol: string;
  endpoint: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationInputTokens: number;
  cacheReadInputTokens: number;
  firstTokenLatencyMs: number;
  streamDurationMs: number;
  userAgent: string;
  upstreamStatusCode: number;
  errorMessage: string;
  traceId: string;
  apiKeyName: string;
  userName: string;
  userEmail: string;
  /** 估算费用（展示单位，null/缺省=未计价） */
  cost?: number | null;
  pricingCurrency?: PricingCurrency;
}

export interface ListAuditLogsRsp extends CommonRsp {
  logs?: AuditLogItem[];
  pageInfo?: PageInfo;
}

// ─── Dashboard Stats ──────────────────────────────────────────

export type Granularity = "minute" | "hour" | "day" | "week";

export interface TrendPoint {
  time: string;
  count: number;
}

export interface ModelTrendItem {
  modelId: string;
  points: TrendPoint[];
}

export interface ModelTrendRsp extends CommonRsp {
  data?: ModelTrendItem[];
}

export interface HarnessTrendItem {
  harness: string;
  points: TrendPoint[];
}

export interface HarnessTrendRsp extends CommonRsp {
  data?: HarnessTrendItem[];
}

export interface RatePoint {
  time: string;
  total: number;
  success: number;
  failed: number;
  successRate: number;
}

export interface RequestRateItem {
  modelId: string;
  points: RatePoint[];
}

export interface RequestRateRsp extends CommonRsp {
  data?: RequestRateItem[];
}

export interface TokenThroughputPoint {
  time: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
}

export interface TokenThroughputRsp extends CommonRsp {
  data?: TokenThroughputPoint[];
}

export interface TokenRatePoint {
  time: string;
  outputTokensPerSecond: number;
}

export interface TokenRateItem {
  modelId: string;
  points: TokenRatePoint[];
}

export interface TokenRateRsp extends CommonRsp {
  data?: TokenRateItem[];
}

export interface ModelUsageItem {
  modelId: string;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheCreationTokens: number;
}

export interface ModelUsageRsp extends CommonRsp {
  data?: ModelUsageItem[];
}

export interface FirstTokenLatencyPoint {
  time: string;
  averageLatencyMs: number;
}

export interface FirstTokenLatencyItem {
  modelId: string;
  points: FirstTokenLatencyPoint[];
}

export interface FirstTokenLatencyRsp extends CommonRsp {
  data?: FirstTokenLatencyItem[];
}

// ─── Audit Option List ───────────────────────────────────────────────────────

export interface AuditOptionListReq {
  field: "user" | "model" | "status" | "ua";
  keyword?: string;
  startTime?: string;
  endTime?: string;
}

export interface AuditOptionListRsp extends CommonRsp {
  items: string[];
}

// ─── Session Option List ─────────────────────────────────────────────────────

export interface SessionOptionListReq {
  field: "score" | "model" | "messageCount" | "harness";
  keyword?: string;
  startTime?: string;
  endTime?: string;
}

export interface SessionOptionListRsp extends CommonRsp {
  items: string[];
}

// ─── Cron ──────────────────────────────────────────────────────────────────────

export interface CronJobItem {
  name: string;
  type: "functional" | "core";
  spec: string;
  description: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ListCronJobsRsp extends CommonRsp {
  jobs?: CronJobItem[];
  pageInfo?: PageInfo;
}

export interface UpdateCronJobReqBody {
  name: string;
  enabled?: boolean;
  spec?: string;
}

// ─── Cron Call Audit ───────────────────────────────────────────────────────────

export interface CronCallAuditItem {
  id: number;
  cronName: string;
  traceId: string;
  startedAt: string;
  endedAt: string;
  durationMs: number;
  status: string;
  triggerSource?: "scheduled" | "manual";
  message: string;
  metadata: Record<string, number>;
  createdAt: string;
}

export interface ListCronCallAuditsRsp extends CommonRsp {
  logs?: CronCallAuditItem[];
  pageInfo?: PageInfo;
}

export interface CronCallAuditOptionListReq {
  field: "type" | "status";
  keyword?: string;
  startTime?: string;
  endTime?: string;
}

export interface CronCallAuditOptionListRsp extends CommonRsp {
  items: string[];
}

// ─── Trigger Words ──────────────────────────────────────────────────────────

export type TriggerAction = "deny" | "omit" | "capture";

export interface CreateTriggerReqBody {
  word: string;
  action?: TriggerAction;
}

export interface UpdateTriggerReqBody {
  action: TriggerAction;
}

export interface TriggerItem {
  id: number;
  word: string;
  action: TriggerAction;
  hitCount: number;
  createdAt: string;
}

export interface ListTriggerRsp extends CommonRsp {
  trigger?: TriggerItem[];
  pageInfo?: PageInfo;
}

export interface DeleteTriggerRsp extends CommonRsp {
  deletedCount?: number;
}

// ─── Metrics ───────────────────────────────────────────────────────────────────

export interface RuntimePoint {
  time: number; // unix 秒（桶起始）
  value: number;
}

export interface RuntimeSeries {
  qps?: RuntimePoint[];
  p95Ms?: RuntimePoint[];
  sseActive?: Record<string, RuntimePoint[]>;
  tokenInput?: RuntimePoint[]; // 输入 token 速率 /s
  tokenOutput?: RuntimePoint[]; // 输出 token 速率 /s
  statusCodes?: Record<string, RuntimePoint[]>; // 状态码 → 该状态码请求数时序（跨 pod 求和）
  instances?: Record<string, RuntimeInstanceSeries>; // pod 名 → 该 pod 曲线
}

export interface RuntimeInstanceSeries {
  goroutines?: RuntimePoint[];
  heapMB?: RuntimePoint[];
  cpuPercent?: RuntimePoint[];
  threads?: RuntimePoint[];
}

export interface RuntimeMetricsRsp extends CommonRsp {
  series: RuntimeSeries;
  latestTime: number;
}

// ─── Dataset ───────────────────────────────────────────────────────────────────

export interface DatasetPreviewRsp extends CommonRsp {
  totalSessions?: number;
  scoreDistribution?: Record<number, number>;
  modelDistribution?: Record<string, number>;
}

export interface DatasetFormatPreviewRsp extends CommonRsp {
  sessionId?: number;
  offset?: number;
  totalCount?: number;
  sharegptJson?: string;
}

export interface DatasetExportSSEStart {
  totalSessions: number;
}

export interface DatasetExportSSEData {
  current: number;
  total: number;
  progress: number;
  json: string;
}

export interface DatasetExportSSEError {
  message: string;
}

export type DatasetExportSSEEvent =
  | { event: "start"; data: DatasetExportSSEStart }
  | { event: "data"; data: DatasetExportSSEData }
  | { event: "done"; data: DatasetExportSSEStart }
  | { event: "error"; data: DatasetExportSSEError };

// ─── Demo Access Audit ────────────────────────────────────────────────────────

export interface DemoAccessAuditItem {
  id: number;
  action: string;
  module: string;
  path: string;
  ip: string;
  userAgent: string;
  reason: string;
  createdAt: string;
}

export interface ListDemoAccessAuditsRsp extends CommonRsp {
  logs?: DemoAccessAuditItem[];
  pageInfo?: PageInfo;
}

export interface DemoAccessAuditOptionListReq {
  field: "action" | "module";
  keyword?: string;
  startTime?: string;
  endTime?: string;
}

export interface DemoAccessAuditOptionListRsp extends CommonRsp {
  items: string[];
}
