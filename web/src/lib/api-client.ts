import { toast } from "sonner";
import { translate } from "@/lib/i18n";
import type {
  CallbackRsp,
  CallbackReqBody,
  LoginRsp,
  RefreshTokenRsp,
  RefreshTokenReqBody,
  DemoStatusRsp,
  DemoLoginRsp,
  GetDemoConfigRsp,
  UpdateDemoConfigReqBody,
  ListDemoSessionsRsp,
  AddDemoSessionsReqBody,
  GetCurUserRsp,
  UpdateUserReqBody,
  ListUsersRsp,
  ListSessionsRsp,
  GetSessionRsp,
  GetSessionMetadataRsp,
  ListSessionMessagesRsp,
  ListSessionToolsRsp,
  ListAPIKeysRsp,
  CreateAPIKeyRsp,
  CreateAPIKeyReqBody,
  ListUpstreamRsp,
  ListModelsPageRsp,
  ModelListSortField,
  ModelCapability,
  CreateEndpointReqBody,
  UpdateEndpointReqBody,
  CreateModelReqBody,
  UpdateModelReqBody,
  OAuth2Provider,
  CreateShareReqBody,
  CreateShareRsp,
  GetShareMetadataRsp,
  ListShareMessagesRsp,
  ListShareToolsRsp,
  ListSharesRsp,
  ListTracesRsp,
  GetTraceRsp,
  ListTraceEventsRsp,
  DeleteTraceRsp,
  CommonRsp,
  ScoreSessionReqBody,
  ScoreSessionRsp,
  ListAuditLogsRsp,
  ModelTrendRsp,
  RequestRateRsp,
  TokenThroughputRsp,
  TokenRateRsp,
  ModelUsageRsp,
  FirstTokenLatencyRsp,
  Granularity,
  DeleteSessionRsp,
  AuditOptionListReq,
  AuditOptionListRsp,
  SessionOptionListReq,
  SessionOptionListRsp,
  CreateTriggerReqBody,
  UpdateTriggerReqBody,
  ListTriggerRsp,
  DeleteTriggerRsp,
  ListCronJobsRsp,
  UpdateCronJobReqBody,
  ListCronCallAuditsRsp,
  CronCallAuditOptionListReq,
  CronCallAuditOptionListRsp,
  ListDemoAccessAuditsRsp,
  DemoAccessAuditOptionListReq,
  DemoAccessAuditOptionListRsp,
  RuntimeMetricsRsp,
  DatasetPreviewRsp,
  DatasetFormatPreviewRsp,
  DatasetExportSSEEvent,
  DatasetExportSSEStart,
  DatasetExportSSEData,
  DatasetExportSSEError,
} from "./types";
import { BusinessErrorCode, type StructuredError, parseError } from "./api-error-handler";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || "";

// Web 分区 API 前缀（与后端 constant.WebAPIPrefix 对应），全部接口路径由此派生，
// 禁止在各请求里重新硬编码（CR M2）。
const API_PREFIX = "/api/web/v1";
const AUTH_TOAST_DURATION_MS = 10_000;

export class ApiError extends Error {
  status: number;
  body: string;
  /** 解析得到的结构化错误（可能为空，如果 body 不是合法 JSON） */
  structured?: StructuredError;

  constructor(status: number, body: string) {
    super(`API error ${status}: ${body}`);
    this.name = "ApiError";
    this.status = status;
    this.body = body;

    // 自动尝试解析 body 中的结构化错误（兼容顶层 {code,message} 与统一 {error:{code,message}}）
    try {
      const parsed = JSON.parse(body);
      if (parsed && typeof parsed === "object") {
        const biz =
          parsed.code !== undefined
            ? (parsed as { code: number; message: string })
            : (parsed as { error?: { code?: number; message?: string } }).error;
        if (biz && biz.code !== undefined && biz.message !== undefined) {
          this.structured = parseError({ code: biz.code, message: biz.message });
          this.structured.httpStatus = status;
          this.structured.rawBody = body;
        }
      }
    } catch {
      // body 不是 JSON，不处理
    }
  }
}

class ApiClient {
  private refreshing: Promise<boolean> | null = null;

  private getHeaders(): HeadersInit {
    const headers: HeadersInit = { "Content-Type": "application/json" };
    if (typeof window !== "undefined") {
      const token = localStorage.getItem("access_token");
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }
      const locale = localStorage.getItem("locale");
      if (locale === "zh" || locale === "en") {
        headers["Accept-Language"] = locale;
      }
    }
    return headers;
  }

  private async tryRefreshToken(): Promise<boolean> {
    if (this.refreshing) return this.refreshing;

    this.refreshing = (async () => {
      const refreshToken = localStorage.getItem("refresh_token");
      if (!refreshToken) return false;

      try {
        const res = await fetch(`${API_BASE}${API_PREFIX}/token/refresh`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refreshToken }),
        });
        // 统一错误契约：刷新接口错误也以 200 + {error} 返回，成功与否只看 accessToken 字段。
        const text = await res.text();
        let data: unknown = null;
        try {
          data = text ? JSON.parse(text) : null;
        } catch {
          data = null;
        }
        const accessToken =
          data && typeof data === "object" ? (data as { accessToken?: unknown }).accessToken : null;
        if (typeof accessToken === "string" && accessToken) {
          localStorage.setItem("access_token", accessToken);
          const refreshToken =
            data && typeof data === "object"
              ? (data as { refreshToken?: unknown }).refreshToken
              : null;
          if (typeof refreshToken === "string" && refreshToken) {
            localStorage.setItem("refresh_token", refreshToken);
          }
          return true;
        }
        return false;
      } catch {
        return false;
      } finally {
        this.refreshing = null;
      }
    })();

    return this.refreshing;
  }

  /**
   * 401/未授权处理：等待（或触发）一次 token 刷新后重试原请求。
   * tryRefreshToken 内部以 Promise 做并发去重，因此并发未授权会共享同一次刷新，
   * 不会出现“一个请求刷新成功、另一个请求误判已重试而强制登出”的竞态。
   * 刷新后仍未授权（refresh token 已失效）才清空凭据并提示重新登录。
   */
  private async handleAuthFailure<T>(path: string, options?: RequestInit): Promise<T> {
    const refreshed = await this.tryRefreshToken();
    if (!refreshed) {
      this.clearAuthAndPromptLogin();
      throw new ApiError(401, "Authentication required");
    }

    const retryRes = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: { ...this.getHeaders(), ...options?.headers },
    });

    if (retryRes.status === 401) {
      this.clearAuthAndPromptLogin();
      throw new ApiError(401, "Authentication required");
    }

    const retryBody = await resolveResponse(retryRes);
    // 统一契约下未授权以 200 + {error:{code:10001}} 返回，同样视为凭据失效
    if (extractBizError(retryBody)?.code === BusinessErrorCode.Unauthorized) {
      this.clearAuthAndPromptLogin();
      throw new ApiError(401, "Authentication required");
    }
    return retryBody as T;
  }

  private clearAuthAndPromptLogin(): void {
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    toast.error(translate("auth.session_expired"), {
      description: translate("auth.please_login"),
      duration: AUTH_TOAST_DURATION_MS,
      action: {
        label: translate("auth.login"),
        onClick: () => {
          window.location.href = "/web/login";
        },
      },
    });
  }

  private async request<T>(path: string, options?: RequestInit): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: { ...this.getHeaders(), ...options?.headers },
    });

    if (res.status === 401) {
      return this.handleAuthFailure<T>(path, options);
    }

    // 统一错误契约：管理 API 除 proxy 外恒返回 HTTP 200，错误语义由 body.error 承载。
    // 只要有合法 error 结构即抛错（10001 未授权除外，走 token 刷新流程）。
    const body = await resolveResponse(res);
    if (extractBizError(body)?.code === BusinessErrorCode.Unauthorized) {
      return this.handleAuthFailure<T>(path, options);
    }

    return body as T;
  }

  // ─── Auth ──────────────────────────────────────────────────────────────────

  async oauth2Login(platform: OAuth2Provider): Promise<LoginRsp> {
    return this.request<LoginRsp>(`${API_PREFIX}/oauth2/login?platform=${platform}`);
  }

  async oauth2Callback(body: CallbackReqBody): Promise<CallbackRsp> {
    return this.request<CallbackRsp>(`${API_PREFIX}/oauth2/callback`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async refreshToken(body: RefreshTokenReqBody): Promise<RefreshTokenRsp> {
    return this.request<RefreshTokenRsp>(`${API_PREFIX}/token/refresh`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  // ─── Demo Account ──────────────────────────────────────────────────────────

  async getDemoStatus(): Promise<DemoStatusRsp> {
    return this.request<DemoStatusRsp>(`${API_PREFIX}/demo/status`);
  }

  async demoLogin(): Promise<DemoLoginRsp> {
    return this.request<DemoLoginRsp>(`${API_PREFIX}/demo/login`, { method: "POST" });
  }

  async getDemoConfig(): Promise<GetDemoConfigRsp> {
    return this.request<GetDemoConfigRsp>(`${API_PREFIX}/demo/config`);
  }

  async updateDemoConfig(body: UpdateDemoConfigReqBody): Promise<GetDemoConfigRsp> {
    return this.request<GetDemoConfigRsp>(`${API_PREFIX}/demo/config`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async listDemoSessions(page = 1, pageSize = 100): Promise<ListDemoSessionsRsp> {
    return this.request<ListDemoSessionsRsp>(
      `${API_PREFIX}/demo/sessions/list?page=${page}&pageSize=${pageSize}`,
    );
  }

  async addDemoSessions(body: AddDemoSessionsReqBody): Promise<ListDemoSessionsRsp> {
    return this.request<ListDemoSessionsRsp>(`${API_PREFIX}/demo/sessions`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async removeDemoSessions(ids: number[]): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/demo/sessions?ids=${ids.join(",")}`, {
      method: "DELETE",
    });
  }

  // ─── User ───────────────────────────────────────────────────────────────────

  async getCurrentUser(): Promise<GetCurUserRsp> {
    return this.request<GetCurUserRsp>(`${API_PREFIX}/user/current`);
  }

  async updateUser(body: UpdateUserReqBody): Promise<GetCurUserRsp> {
    return this.request<GetCurUserRsp>(`${API_PREFIX}/user`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async listUsers(
    page: number,
    pageSize: number,
    opts?: { query?: string; permission?: string },
  ): Promise<ListUsersRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (opts?.query) params.set("query", opts.query);
    if (opts?.permission) params.set("permission", opts.permission);
    return this.request<ListUsersRsp>(`${API_PREFIX}/user/list?${params}`);
  }

  async approveUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/user/approve?id=${id}`, { method: "POST" });
  }

  async demoteUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/user/demote?id=${id}`, { method: "POST" });
  }

  async deleteUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/user/delete?id=${id}`, { method: "DELETE" });
  }

  async setDemoUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/user/demo?id=${id}`, { method: "POST" });
  }

  async restoreDemoUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/user/demo/restore?id=${id}`, { method: "POST" });
  }

  // ─── Session (JWT auth) ────────────────────────────────────────────────────

  async listSessions(params: {
    page: number;
    pageSize: number;
    sort?: string;
    sortField?: string;
    startTime?: string;
    endTime?: string;
    keyword?: string;
    filter?: string;
  }): Promise<ListSessionsRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.sort) sp.set("sort", params.sort);
    if (params.sortField) sp.set("sortField", params.sortField);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.filter) sp.set("filter", params.filter);
    return this.request<ListSessionsRsp>(`${API_PREFIX}/session/list?${sp}`);
  }

  async listSessionOptions(params: SessionOptionListReq): Promise<SessionOptionListRsp> {
    const sp = new URLSearchParams();
    sp.set("field", params.field);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<SessionOptionListRsp>(`${API_PREFIX}/session/option/list?${sp}`);
  }

  async getSession(sessionId: number): Promise<GetSessionRsp> {
    return this.request<GetSessionRsp>(`${API_PREFIX}/session?id=${sessionId}`);
  }

  async getSessionMetadata(sessionId: number): Promise<GetSessionMetadataRsp> {
    return this.request<GetSessionMetadataRsp>(`${API_PREFIX}/session/metadata?id=${sessionId}`);
  }

  async listSessionMessages(
    sessionId: number,
    page: number = 1,
    pageSize: number = 50,
  ): Promise<ListSessionMessagesRsp> {
    return this.request<ListSessionMessagesRsp>(
      `${API_PREFIX}/session/message/list?id=${sessionId}&page=${page}&pageSize=${pageSize}`,
    );
  }

  async listSessionTools(
    sessionId: number,
    page: number = 1,
    pageSize: number = 20,
  ): Promise<ListSessionToolsRsp> {
    return this.request<ListSessionToolsRsp>(
      `${API_PREFIX}/session/tool/list?id=${sessionId}&page=${page}&pageSize=${pageSize}`,
    );
  }

  // ─── Session Score ─────────────────────────────────────────────────────────

  async scoreSession(body: ScoreSessionReqBody): Promise<ScoreSessionRsp> {
    return this.request<ScoreSessionRsp>(`${API_PREFIX}/session/score`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async deleteScoreSession(sessionId: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/session/score?id=${sessionId}`, {
      method: "DELETE",
    });
  }

  // ─── Session Share ─────────────────────────────────────────────────────────

  async createShare(body: CreateShareReqBody): Promise<CreateShareRsp> {
    return this.request<CreateShareRsp>(`${API_PREFIX}/session/share`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async listShares(page: number = 1, pageSize: number = 20): Promise<ListSharesRsp> {
    return this.request<ListSharesRsp>(
      `${API_PREFIX}/session/share/list?page=${page}&pageSize=${pageSize}`,
    );
  }

  async deleteShare(shareId: string): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/session/share?id=${encodeURIComponent(shareId)}`, {
      method: "DELETE",
    });
  }

  // ─── Session Delete ──────────────────────────────────────────────────────

  async deleteSession(sessionId: number): Promise<DeleteSessionRsp> {
    return this.request<DeleteSessionRsp>(`${API_PREFIX}/session?ids=${sessionId}`, {
      method: "DELETE",
    });
  }

  async batchDeleteSessions(ids: number[]): Promise<DeleteSessionRsp> {
    return this.request<DeleteSessionRsp>(`${API_PREFIX}/session?ids=${ids.join(",")}`, {
      method: "DELETE",
    });
  }

  /**
   * 公开只读接口（无需鉴权），仅携带 Accept-Language。
   * 同样遵循统一错误契约：body 携带 error 结构即抛错。
   */
  private async publicGet<T>(path: string): Promise<T> {
    const headers: HeadersInit = { "Content-Type": "application/json" };
    const locale = typeof window !== "undefined" ? localStorage.getItem("locale") : null;
    if (locale === "zh" || locale === "en") headers["Accept-Language"] = locale;
    const res = await fetch(`${API_BASE}${path}`, { method: "GET", headers });
    const body = await resolveResponse(res);
    return body as T;
  }

  /**
   * Get shared session metadata (public, no auth).
   */
  async getShareMetadata(shareId: string): Promise<GetShareMetadataRsp> {
    return this.publicGet<GetShareMetadataRsp>(
      `${API_PREFIX}/session/share/metadata?id=${encodeURIComponent(shareId)}`,
    );
  }

  /**
   * List shared session messages with pagination (public, no auth).
   */
  async listShareMessages(
    shareId: string,
    page: number = 1,
    pageSize: number = 50,
  ): Promise<ListShareMessagesRsp> {
    return this.publicGet<ListShareMessagesRsp>(
      `${API_PREFIX}/session/share/message/list?id=${encodeURIComponent(shareId)}&page=${page}&pageSize=${pageSize}`,
    );
  }

  /**
   * List shared session tools with pagination (public, no auth).
   */
  async listShareTools(
    shareId: string,
    page: number = 1,
    pageSize: number = 20,
  ): Promise<ListShareToolsRsp> {
    return this.publicGet<ListShareToolsRsp>(
      `${API_PREFIX}/session/share/tool/list?id=${encodeURIComponent(shareId)}&page=${page}&pageSize=${pageSize}`,
    );
  }

  // ─── API Keys ──────────────────────────────────────────────────────────────

  async listAPIKeys(
    page: number = 1,
    pageSize: number = 20,
    query?: string,
  ): Promise<ListAPIKeysRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListAPIKeysRsp>(`${API_PREFIX}/apikey/list?${params}`);
  }

  async createAPIKey(body: CreateAPIKeyReqBody): Promise<CreateAPIKeyRsp> {
    return this.request<CreateAPIKeyRsp>(`${API_PREFIX}/apikey`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async deleteAPIKey(id: number): Promise<void> {
    await this.request(`${API_PREFIX}/apikey?id=${id}`, {
      method: "DELETE",
    });
  }

  // ─── Upstream (endpoint 分组视图) ──────────────────────────────────────────

  async listUpstream(
    page: number = 1,
    pageSize: number = 10,
    query?: string,
    username?: string,
  ): Promise<ListUpstreamRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    if (username) params.set("username", username);
    return this.request<ListUpstreamRsp>(`${API_PREFIX}/upstream/list?${params}`);
  }

  /**
   * 平铺模型列表（独立于分组视图的真分页）。
   * sortField 必须在 ModelListSortField 白名单内；后端对非法值静默回退默认列。
   */
  async listModelsPage(params: {
    page: number;
    pageSize: number;
    query?: string;
    sortField?: ModelListSortField;
    sort?: "asc" | "desc";
    status?: "enabled" | "disabled";
    endpointID?: number;
    capability?: ModelCapability;
    username?: string;
  }): Promise<ListModelsPageRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.query) sp.set("query", params.query);
    if (params.sortField) sp.set("sortField", params.sortField);
    if (params.sort) sp.set("sort", params.sort);
    if (params.status) sp.set("status", params.status);
    if (params.endpointID) sp.set("endpointID", String(params.endpointID));
    if (params.capability) sp.set("capability", params.capability);
    if (params.username) sp.set("username", params.username);
    return this.request<ListModelsPageRsp>(`${API_PREFIX}/model/list?${sp}`);
  }

  // ─── Endpoints (admin) ─────────────────────────────────────────────────────

  async createEndpoint(body: CreateEndpointReqBody): Promise<void> {
    await this.request(`${API_PREFIX}/endpoint`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async updateEndpoint(id: number, body: UpdateEndpointReqBody): Promise<void> {
    await this.request(`${API_PREFIX}/endpoint?id=${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async deleteEndpoint(id: number): Promise<void> {
    await this.request(`${API_PREFIX}/endpoint?id=${id}`, {
      method: "DELETE",
    });
  }

  // ─── Models (admin) ────────────────────────────────────────────────────────

  async createModel(body: CreateModelReqBody): Promise<void> {
    await this.request(`${API_PREFIX}/model`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async updateModel(id: number, body: UpdateModelReqBody): Promise<void> {
    await this.request(`${API_PREFIX}/model?id=${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async deleteModel(id: number): Promise<void> {
    await this.request(`${API_PREFIX}/model?id=${id}`, {
      method: "DELETE",
    });
  }

  // ─── Audit (admin / user) ──────────────────────────────────────────────────

  async listAuditLogs(params: {
    page: number;
    pageSize: number;
    query?: string;
    sort?: string;
    sortField?: string;
    startTime?: string;
    endTime?: string;
    filter?: string;
  }): Promise<ListAuditLogsRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.query) sp.set("query", params.query);
    if (params.sort) sp.set("sort", params.sort);
    if (params.sortField) sp.set("sortField", params.sortField);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    if (params.filter) sp.set("filter", params.filter);
    return this.request<ListAuditLogsRsp>(`${API_PREFIX}/audit/model/log/list?${sp}`);
  }

  async listAuditOptions(params: AuditOptionListReq): Promise<AuditOptionListRsp> {
    const sp = new URLSearchParams();
    sp.set("field", params.field);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<AuditOptionListRsp>(`${API_PREFIX}/audit/model/option/list?${sp}`);
  }

  async fetchModelTrend(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<ModelTrendRsp> {
    const sp = new URLSearchParams(params);
    return this.request<ModelTrendRsp>(`${API_PREFIX}/audit/stats/model/trend?${sp}`);
  }

  async fetchRequestRate(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<RequestRateRsp> {
    const sp = new URLSearchParams(params);
    return this.request<RequestRateRsp>(`${API_PREFIX}/audit/stats/request/rate?${sp}`);
  }

  async fetchTokenThroughput(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<TokenThroughputRsp> {
    const sp = new URLSearchParams(params);
    return this.request<TokenThroughputRsp>(`${API_PREFIX}/audit/stats/token/throughput?${sp}`);
  }

  async fetchTokenRate(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<TokenRateRsp> {
    const sp = new URLSearchParams(params);
    return this.request<TokenRateRsp>(`${API_PREFIX}/audit/stats/token/rate?${sp}`);
  }

  async fetchModelUsage(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<ModelUsageRsp> {
    const sp = new URLSearchParams(params);
    return this.request<ModelUsageRsp>(`${API_PREFIX}/audit/stats/model/usage?${sp}`);
  }

  async fetchFirstTokenLatency(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<FirstTokenLatencyRsp> {
    const sp = new URLSearchParams(params);
    return this.request<FirstTokenLatencyRsp>(`${API_PREFIX}/audit/stats/token/latency?${sp}`);
  }

  // ─── Trigger Words ─────────────────────────────────────────────────────

  async createTrigger(body: CreateTriggerReqBody): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/trigger`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async listTrigger(page: number, pageSize: number, query?: string): Promise<ListTriggerRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListTriggerRsp>(`${API_PREFIX}/trigger/list?${params}`);
  }

  async deleteTrigger(id: number): Promise<DeleteTriggerRsp> {
    return this.request<DeleteTriggerRsp>(`${API_PREFIX}/trigger?ids=${id}`, { method: "DELETE" });
  }

  async batchDeleteTrigger(ids: number[]): Promise<DeleteTriggerRsp> {
    return this.request<DeleteTriggerRsp>(`${API_PREFIX}/trigger?ids=${ids.join(",")}`, {
      method: "DELETE",
    });
  }

  async updateTrigger(id: number, body: UpdateTriggerReqBody): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/trigger?id=${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  // ─── Trace (codex hooks) ─────────────────────────────────────────────────────

  async listTraces(
    page: number = 1,
    pageSize: number = 20,
    query?: string,
  ): Promise<ListTracesRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListTracesRsp>(`${API_PREFIX}/trace/list?${params}`);
  }

  async getTrace(id: number): Promise<GetTraceRsp> {
    return this.request<GetTraceRsp>(`${API_PREFIX}/trace?id=${id}`);
  }

  async listTraceEvents(
    traceId: number,
    page: number = 1,
    pageSize: number = 50,
  ): Promise<ListTraceEventsRsp> {
    const params = new URLSearchParams({
      id: String(traceId),
      page: String(page),
      pageSize: String(pageSize),
    });
    return this.request<ListTraceEventsRsp>(`${API_PREFIX}/trace/event/list?${params}`);
  }

  async deleteTrace(traceId: number): Promise<DeleteTraceRsp> {
    return this.request<DeleteTraceRsp>(`${API_PREFIX}/trace?ids=${traceId}`, { method: "DELETE" });
  }

  async batchDeleteTraces(ids: number[]): Promise<DeleteTraceRsp> {
    return this.request<DeleteTraceRsp>(`${API_PREFIX}/trace?ids=${ids.join(",")}`, {
      method: "DELETE",
    });
  }

  // ─── Cron (admin) ──────────────────────────────────────────────────────────

  async listCronJobs(params: {
    page: number;
    pageSize: number;
    query?: string;
    sort?: string;
    sortField?: string;
  }): Promise<ListCronJobsRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.query) sp.set("query", params.query);
    if (params.sort) sp.set("sort", params.sort);
    if (params.sortField) sp.set("sortField", params.sortField);
    return this.request<ListCronJobsRsp>(`${API_PREFIX}/cron/list?${sp}`);
  }

  async updateCronJob(body: UpdateCronJobReqBody): Promise<CommonRsp> {
    const payload: Record<string, unknown> = {};
    if (body.enabled !== undefined) payload.enabled = body.enabled;
    if (body.spec !== undefined) payload.spec = body.spec;
    return this.request<CommonRsp>(`${API_PREFIX}/cron?name=${encodeURIComponent(body.name)}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    });
  }

  async triggerCronJob(name: string): Promise<CommonRsp> {
    return this.request<CommonRsp>(`${API_PREFIX}/cron/trigger?name=${encodeURIComponent(name)}`, {
      method: "POST",
    });
  }

  async listCronCallAudits(params: {
    page: number;
    pageSize: number;
    query?: string;
    sort?: string;
    sortField?: string;
    startTime?: string;
    endTime?: string;
    filter?: string;
  }): Promise<ListCronCallAuditsRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.query) sp.set("query", params.query);
    if (params.sort) sp.set("sort", params.sort);
    if (params.sortField) sp.set("sortField", params.sortField);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    if (params.filter) sp.set("filter", params.filter);
    return this.request<ListCronCallAuditsRsp>(`${API_PREFIX}/audit/cron/log/list?${sp}`);
  }

  async listCronCallAuditOptions(
    params: CronCallAuditOptionListReq,
  ): Promise<CronCallAuditOptionListRsp> {
    const sp = new URLSearchParams();
    sp.set("field", params.field);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<CronCallAuditOptionListRsp>(`${API_PREFIX}/audit/cron/option/list?${sp}`);
  }

  async listDemoAccessAudits(params: {
    page: number;
    pageSize: number;
    query?: string;
    sort?: string;
    sortField?: string;
    startTime?: string;
    endTime?: string;
    filter?: string;
  }): Promise<ListDemoAccessAuditsRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.query) sp.set("query", params.query);
    if (params.sort) sp.set("sort", params.sort);
    if (params.sortField) sp.set("sortField", params.sortField);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    if (params.filter) sp.set("filter", params.filter);
    return this.request<ListDemoAccessAuditsRsp>(`${API_PREFIX}/audit/demo/log/list?${sp}`);
  }

  async listDemoAccessAuditOptions(
    params: DemoAccessAuditOptionListReq,
  ): Promise<DemoAccessAuditOptionListRsp> {
    const sp = new URLSearchParams();
    sp.set("field", params.field);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<DemoAccessAuditOptionListRsp>(`${API_PREFIX}/audit/demo/option/list?${sp}`);
  }

  async getRuntimeMetrics(params: { range: string; since?: number }): Promise<RuntimeMetricsRsp> {
    const sp = new URLSearchParams({ range: params.range });
    if (params.since && params.since > 0) sp.set("since", String(params.since));
    return this.request<RuntimeMetricsRsp>(`${API_PREFIX}/metrics/runtime?${sp}`);
  }

  // ─── Dataset ───────────────────────────────────────────────────────────────────

  async previewDataset(params: {
    minScore?: number;
    modelIds?: string[];
    startTime?: string;
    endTime?: string;
  }): Promise<DatasetPreviewRsp> {
    const sp = new URLSearchParams();
    if (params.minScore) sp.set("minScore", String(params.minScore));
    if (params.modelIds && params.modelIds.length > 0)
      sp.set("modelIds", params.modelIds.join(","));
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<DatasetPreviewRsp>(`${API_PREFIX}/dataset/preview?${sp}`);
  }

  async previewDatasetFormat(params: {
    minScore?: number;
    modelIds?: string[];
    startTime?: string;
    endTime?: string;
    offset?: number;
  }): Promise<DatasetFormatPreviewRsp> {
    const sp = new URLSearchParams();
    if (params.minScore) sp.set("minScore", String(params.minScore));
    if (params.modelIds && params.modelIds.length > 0)
      sp.set("modelIds", params.modelIds.join(","));
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    if (params.offset !== undefined) sp.set("offset", String(params.offset));
    return this.request<DatasetFormatPreviewRsp>(`${API_PREFIX}/dataset/sample?${sp}`);
  }

  async exportDatasetStream(
    params: {
      minScore?: number;
      modelIds?: string[];
      startTime?: string;
      endTime?: string;
    },
    onEvent: (event: DatasetExportSSEEvent) => void,
    signal?: AbortSignal,
  ): Promise<void> {
    const sp = new URLSearchParams();
    if (params.minScore) sp.set("minScore", String(params.minScore));
    if (params.modelIds && params.modelIds.length > 0)
      sp.set("modelIds", params.modelIds.join(","));
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);

    const doFetch = () =>
      fetch(`${API_BASE}${API_PREFIX}/dataset/export?${sp}`, {
        headers: { ...this.getHeaders() },
        signal,
      });

    let res = await doFetch();

    if (res.status === 401) {
      const refreshed = await this.tryRefreshToken();
      if (refreshed) {
        res = await doFetch();
      } else {
        this.clearAuthAndPromptLogin();
        throw new ApiError(401, "Authentication required");
      }
    }

    // 统一错误契约：管理 API 错误以 200 + {error} JSON 返回，而非 SSE 帧。
    // 非 text/event-stream 的响应说明握手阶段即失败（如权限不足），直接抛错。
    const contentType = res.headers.get("content-type") ?? "";
    if (!contentType.includes("text/event-stream")) {
      await resolveResponse(res);
      return;
    }

    if (!res.body) throw new ApiError(500, "No response body");

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        const frames = buffer.split("\n\n");
        buffer = frames.pop() ?? "";

        for (const frame of frames) {
          const event = parseSSEFrame(frame);
          if (event) onEvent(event);
        }
      }

      if (buffer.trim()) {
        const event = parseSSEFrame(buffer);
        if (event) onEvent(event);
      }
    } finally {
      reader.releaseLock();
    }
  }
}

/**
 * 从响应体中提取统一业务错误结构；body 无 error 或 error 为 null（成功）时返回 null。
 * 管理 API 统一错误契约：HTTP 200 + {error:{code,message}}，成功时 error 为 null 或缺失。
 */
function extractBizError(body: unknown): { code: number; message: string } | null {
  if (!body || typeof body !== "object") return null;
  const err = (body as { error?: unknown }).error;
  if (!err || typeof err !== "object") return null;
  const biz = err as { code?: unknown; message?: unknown };
  if (typeof biz.code === "number" && typeof biz.message === "string") {
    return { code: biz.code, message: biz.message };
  }
  return null;
}

/**
 * 读取响应并按统一错误契约解析：
 * - body 携带合法 error 结构 → 抛 ApiError（10001 未授权豁免，由调用方决定刷新/登出流程）
 * - 非 2xx 且无 error 结构（理论仅在异常路径出现）→ 抛 ApiError
 * - 其余情况返回解析后的 body（可能为 null，如空 body / 非 JSON）
 */
async function resolveResponse(res: Response): Promise<unknown> {
  const text = await res.text();
  let body: unknown = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    body = null;
  }
  const bizErr = extractBizError(body);
  if (bizErr && bizErr.code !== BusinessErrorCode.Unauthorized) {
    throw new ApiError(res.status, text);
  }
  if (!res.ok) {
    throw new ApiError(res.status, text);
  }
  return body;
}

function parseSSEFrame(frame: string): DatasetExportSSEEvent | null {
  const lines = frame.split("\n");
  let event = "";
  const dataLines: string[] = [];

  for (const line of lines) {
    if (line.startsWith(":")) continue; // SSE 注释行，忽略
    if (line.startsWith("event:")) {
      event = line.slice("event:".length).trim();
    } else if (line.startsWith("data:")) {
      // 规范：冒号后至多一个前导空格；同事件的多个 data: 行以 \n 拼接
      dataLines.push(line.slice("data:".length).replace(/^ /, ""));
    }
  }

  if (!event || dataLines.length === 0) return null;
  const data = dataLines.join("\n");

  try {
    const parsed = JSON.parse(data);
    switch (event) {
      case "start":
        return { event: "start", data: parsed as DatasetExportSSEStart };
      case "data":
        return { event: "data", data: parsed as DatasetExportSSEData };
      case "done":
        return { event: "done", data: parsed as DatasetExportSSEStart };
      case "error":
        return { event: "error", data: parsed as DatasetExportSSEError };
      default:
        return null;
    }
  } catch {
    return null;
  }
}

export const api = new ApiClient();
