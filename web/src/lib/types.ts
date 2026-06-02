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

export type Permission = "pending" | "user" | "admin";

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

export interface UpdateUserReqBody {
  user: User;
}

// ─── Session ───────────────────────────────────────────────────────────────────

export interface SessionSummary {
  id: number;
  createdAt: string;
  updatedAt: string;
  summary: string;
  messageCount: number;
  toolCount: number;
  metadata?: Record<string, string>;
}

export interface SessionDetail {
  id: number;
  apiKeyName: string;
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
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
}

export interface MessageItem {
  id: number;
  model: string;
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
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
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

// ─── Session Share ─────────────────────────────────────────────────────────────

export interface CreateShareReqBody {
  sessionId: number;
  expiresIn?: string;
  expiresAt?: number;
}

export interface CreateShareRsp extends CommonRsp {
  shareId?: string;
  expiresAt?: string;
}

// ─── Share 分页接口（公开，与 session detail 优化模式对齐） ───────────────────

export interface ShareSessionMetadata {
  id: number;
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
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

export interface APIKeyItem {
  id: number;
  name: string;
  key: string; // masked
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

export interface EndpointItem {
  id: number;
  name: string;
  openaiBaseURL: string;
  anthropicBaseURL: string;
  maskedAPIKey: string;
  supportOpenAIChatCompletion: boolean;
  supportOpenAIResponse: boolean;
  supportAnthropicMessage: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateEndpointReqBody {
  name: string;
  openaiBaseURL?: string;
  anthropicBaseURL?: string;
  apiKey: string;
  supportOpenAIChatCompletion?: boolean;
  supportOpenAIResponse?: boolean;
  supportAnthropicMessage?: boolean;
}

export interface UpdateEndpointReqBody {
  name?: string;
  openaiBaseURL?: string;
  anthropicBaseURL?: string;
  apiKey?: string;
  supportOpenAIChatCompletion?: boolean;
  supportOpenAIResponse?: boolean;
  supportAnthropicMessage?: boolean;
}

export interface ListEndpointsRsp extends CommonRsp {
  endpoints?: EndpointItem[];
  pageInfo?: PageInfo;
}

// ─── Model ─────────────────────────────────────────────────────────────────────

export interface ModelItem {
  id: number;
  alias: string;
  modelName: string;
  endpoint: EndpointItem;
  createdAt: string;
  updatedAt: string;
}

export interface CreateModelReqBody {
  alias: string;
  modelName: string;
  endpointID: number;
}

export interface UpdateModelReqBody {
  alias?: string;
  modelName?: string;
  endpointID?: number;
}

export interface ListModelsRsp extends CommonRsp {
  models?: ModelItem[];
  pageInfo?: PageInfo;
}

// ─── Audit ─────────────────────────────────────────────────────────────────────

export interface AuditLogItem {
  id: number;
  createdAt: string;
  model: string;
  upstreamProvider: string;
  apiProvider: string;
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
  model: string;
  points: TrendPoint[];
}

export interface ModelTrendRsp extends CommonRsp {
  data?: ModelTrendItem[];
}

export interface RatePoint {
  time: string;
  total: number;
  success: number;
  failed: number;
  successRate: number;
}

export interface RequestRateItem {
  model: string;
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
  outputTokensPerSecond: number;
}

export interface TokenThroughputItem {
  model: string;
  points: TokenThroughputPoint[];
}

export interface TokenThroughputRsp extends CommonRsp {
  data?: TokenThroughputItem[];
}

export interface TokenRatePoint {
  time: string;
  outputTokensPerSecond: number;
}

export interface TokenRateItem {
  model: string;
  points: TokenRatePoint[];
}

export interface TokenRateRsp extends CommonRsp {
  data?: TokenRateItem[];
}

export interface TokenUsageItem {
  model: string;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheCreationTokens: number;
}

export interface TokenUsageRsp extends CommonRsp {
  data?: TokenUsageItem[];
}
