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
  messageIds: number[];
  toolIds: number[];
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
  isShared?: boolean;
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

// ─── Session Share ─────────────────────────────────────────────────────────────

/**
 * Public-facing session detail returned by the share-content endpoint.
 * Sensitive fields (apiKeyName, ...) are stripped on the backend.
 */
export interface ShareContentSessionDetail {
  id: number;
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
  messages: MessageItem[];
  tools: ToolItem[];
}

export interface CreateShareReqBody {
  sessionId: number;
}

export interface CreateShareRsp extends CommonRsp {
  shareId?: string;
  expiresAt?: string;
}

export interface GetShareContentRsp extends CommonRsp {
  session?: ShareContentSessionDetail;
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
  endpointID: number;
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
