import { toast } from "sonner";
import { translate } from "@/lib/i18n";
import type {
  CallbackRsp,
  CallbackReqBody,
  LoginRsp,
  RefreshTokenRsp,
  RefreshTokenReqBody,
  GetCurUserRsp,
  UpdateUserReqBody,
  ListSessionsRsp,
  GetSessionRsp,
  GetSessionMetadataRsp,
  ListSessionMessagesRsp,
  ListSessionToolsRsp,
  ListAPIKeysRsp,
  CreateAPIKeyRsp,
  CreateAPIKeyReqBody,
  ListEndpointsRsp,
  CreateEndpointReqBody,
  UpdateEndpointReqBody,
  ListModelsRsp,
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
  CreateBlockedReqBody,
  ListBlockedRsp,
  ListCronJobsRsp,
  UpdateCronJobReqBody,
  ListCronCallAuditsRsp,
  CronCallAuditOptionListReq,
  CronCallAuditOptionListRsp,
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
        const res = await fetch(`${API_BASE}/api/v1/token/refresh`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refreshToken }),
        });
        if (!res.ok) return false;
        const data = await res.json();
        if (data.accessToken) {
          localStorage.setItem("access_token", data.accessToken);
          if (data.refreshToken) {
            localStorage.setItem("refresh_token", data.refreshToken);
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
   * 401 处理：等待（或触发）一次 token 刷新后重试原请求。
   * tryRefreshToken 内部以 Promise 做并发去重，因此并发 401 会共享同一次刷新，
   * 不会出现“一个请求刷新成功、另一个请求误判已重试而强制登出”的竞态。
   * 刷新后仍 401（refresh token 已失效）才清空凭据并提示重新登录。
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
    if (!retryRes.ok) {
      throw new ApiError(retryRes.status, await retryRes.text());
    }

    const retryBody = await retryRes.json();
    // 业务层未授权（HTTP 200 包装）同样视为凭据失效
    if (retryBody && typeof retryBody === "object" && retryBody.error?.code === BusinessErrorCode.Unauthorized) {
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

  private async request<T>(
    path: string,
    options?: RequestInit
  ): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: { ...this.getHeaders(), ...options?.headers },
    });

    if (res.status === 401) {
      return this.handleAuthFailure<T>(path, options);
    }

    if (!res.ok) {
      throw new ApiError(res.status, await res.text());
    }

    const body = await res.json();

    // Unified response: business-level auth error returned with HTTP 200
    if (body && typeof body === "object" && body.error?.code === BusinessErrorCode.Unauthorized) {
      return this.handleAuthFailure<T>(path, options);
    }

    return body as T;
  }

  // ─── Auth ──────────────────────────────────────────────────────────────────

  async oauth2Login(platform: OAuth2Provider): Promise<LoginRsp> {
    return this.request<LoginRsp>(`/api/v1/oauth2/login?platform=${platform}`);
  }

  async oauth2Callback(body: CallbackReqBody): Promise<CallbackRsp> {
    return this.request<CallbackRsp>("/api/v1/oauth2/callback", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async refreshToken(body: RefreshTokenReqBody): Promise<RefreshTokenRsp> {
    return this.request<RefreshTokenRsp>("/api/v1/token/refresh", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  // ─── User ───────────────────────────────────────────────────────────────────

  async getCurrentUser(): Promise<GetCurUserRsp> {
    return this.request<GetCurUserRsp>("/api/v1/user/current");
  }

  async updateUser(body: UpdateUserReqBody): Promise<GetCurUserRsp> {
    return this.request<GetCurUserRsp>("/api/v1/user", {
      method: "PATCH",
      body: JSON.stringify(body),
    });
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
    return this.request<ListSessionsRsp>(`/api/v1/session/list?${sp}`);
  }

  async listSessionOptions(params: SessionOptionListReq): Promise<SessionOptionListRsp> {
    const sp = new URLSearchParams();
    sp.set("field", params.field);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<SessionOptionListRsp>(`/api/v1/session/option/list?${sp}`);
  }

  async getSession(sessionId: number): Promise<GetSessionRsp> {
    return this.request<GetSessionRsp>(
      `/api/v1/session?id=${sessionId}`
    );
  }

  async getSessionMetadata(sessionId: number): Promise<GetSessionMetadataRsp> {
    return this.request<GetSessionMetadataRsp>(
      `/api/v1/session/metadata?id=${sessionId}`
    );
  }

  async listSessionMessages(
    sessionId: number,
    page: number = 1,
    pageSize: number = 50
  ): Promise<ListSessionMessagesRsp> {
    return this.request<ListSessionMessagesRsp>(
      `/api/v1/session/message/list?id=${sessionId}&page=${page}&pageSize=${pageSize}`
    );
  }

  async listSessionTools(
    sessionId: number,
    page: number = 1,
    pageSize: number = 20
  ): Promise<ListSessionToolsRsp> {
    return this.request<ListSessionToolsRsp>(
      `/api/v1/session/tool/list?id=${sessionId}&page=${page}&pageSize=${pageSize}`
    );
  }

  // ─── Session Score ─────────────────────────────────────────────────────────

  async scoreSession(body: ScoreSessionReqBody): Promise<ScoreSessionRsp> {
    return this.request<ScoreSessionRsp>("/api/v1/session/score", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async deleteScoreSession(sessionId: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`/api/v1/session/score?id=${sessionId}`, {
      method: "DELETE",
    });
  }

  // ─── Session Share ─────────────────────────────────────────────────────────

  async createShare(body: CreateShareReqBody): Promise<CreateShareRsp> {
    return this.request<CreateShareRsp>("/api/v1/session/share", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async listShares(
    page: number = 1,
    pageSize: number = 20
  ): Promise<ListSharesRsp> {
    return this.request<ListSharesRsp>(
      `/api/v1/session/share/list?page=${page}&pageSize=${pageSize}`
    );
  }

  async deleteShare(shareId: string): Promise<CommonRsp> {
    return this.request<CommonRsp>(
      `/api/v1/session/share?id=${encodeURIComponent(shareId)}`,
      { method: "DELETE" }
    );
  }

  // ─── Session Delete ──────────────────────────────────────────────────────

  async deleteSession(sessionId: number): Promise<DeleteSessionRsp> {
    return this.request<DeleteSessionRsp>(
      `/api/v1/session?ids=${sessionId}`,
      { method: "DELETE" }
    );
  }

  async batchDeleteSessions(ids: number[]): Promise<DeleteSessionRsp> {
    return this.request<DeleteSessionRsp>(
      `/api/v1/session?ids=${ids.join(",")}`,
      { method: "DELETE" }
    );
  }

  /**
   * 公开只读接口（无需鉴权），仅携带 Accept-Language。
   */
  private async publicGet<T>(path: string): Promise<T> {
    const headers: HeadersInit = { "Content-Type": "application/json" };
    const locale = typeof window !== "undefined" ? localStorage.getItem("locale") : null;
    if (locale === "zh" || locale === "en") headers["Accept-Language"] = locale;
    const res = await fetch(`${API_BASE}${path}`, { method: "GET", headers });
    if (!res.ok) {
      throw new ApiError(res.status, await res.text());
    }
    return res.json();
  }

  /**
   * Get shared session metadata (public, no auth).
   */
  async getShareMetadata(shareId: string): Promise<GetShareMetadataRsp> {
    return this.publicGet<GetShareMetadataRsp>(
      `/api/v1/session/share/metadata?id=${encodeURIComponent(shareId)}`
    );
  }

  /**
   * List shared session messages with pagination (public, no auth).
   */
  async listShareMessages(
    shareId: string,
    page: number = 1,
    pageSize: number = 50
  ): Promise<ListShareMessagesRsp> {
    return this.publicGet<ListShareMessagesRsp>(
      `/api/v1/session/share/message/list?id=${encodeURIComponent(shareId)}&page=${page}&pageSize=${pageSize}`
    );
  }

  /**
   * List shared session tools with pagination (public, no auth).
   */
  async listShareTools(
    shareId: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<ListShareToolsRsp> {
    return this.publicGet<ListShareToolsRsp>(
      `/api/v1/session/share/tool/list?id=${encodeURIComponent(shareId)}&page=${page}&pageSize=${pageSize}`
    );
  }

  // ─── API Keys ──────────────────────────────────────────────────────────────

  async listAPIKeys(
    page: number = 1,
    pageSize: number = 20,
    query?: string
  ): Promise<ListAPIKeysRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListAPIKeysRsp>(`/api/v1/apikey/list?${params}`);
  }

  async createAPIKey(
    body: CreateAPIKeyReqBody
  ): Promise<CreateAPIKeyRsp> {
    return this.request<CreateAPIKeyRsp>("/api/v1/apikey", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async deleteAPIKey(id: number): Promise<void> {
    await this.request(`/api/v1/apikey?id=${id}`, {
      method: "DELETE",
    });
  }

  // ─── Endpoints (admin) ─────────────────────────────────────────────────────

  async listEndpoints(
    page: number = 1,
    pageSize: number = 20,
    query?: string
  ): Promise<ListEndpointsRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListEndpointsRsp>(`/api/v1/endpoint/list?${params}`);
  }

  async createEndpoint(
    body: CreateEndpointReqBody
  ): Promise<ListEndpointsRsp> {
    return this.request<ListEndpointsRsp>("/api/v1/endpoint", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async updateEndpoint(
    id: number,
    body: UpdateEndpointReqBody
  ): Promise<ListEndpointsRsp> {
    return this.request<ListEndpointsRsp>(`/api/v1/endpoint?id=${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async deleteEndpoint(id: number): Promise<void> {
    await this.request(`/api/v1/endpoint?id=${id}`, {
      method: "DELETE",
    });
  }

  // ─── Models (admin) ────────────────────────────────────────────────────────

  async listModels(
    page: number = 1,
    pageSize: number = 20,
    query?: string
  ): Promise<ListModelsRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListModelsRsp>(`/api/v1/model/list?${params}`);
  }

  async createModel(body: CreateModelReqBody): Promise<ListModelsRsp> {
    return this.request<ListModelsRsp>("/api/v1/model", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async updateModel(
    id: number,
    body: UpdateModelReqBody
  ): Promise<ListModelsRsp> {
    return this.request<ListModelsRsp>(`/api/v1/model?id=${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async deleteModel(id: number): Promise<void> {
    await this.request(`/api/v1/model?id=${id}`, {
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
    return this.request<ListAuditLogsRsp>(`/api/v1/audit/model/log/list?${sp}`);
  }

  async listAuditOptions(params: AuditOptionListReq): Promise<AuditOptionListRsp> {
    const sp = new URLSearchParams();
    sp.set("field", params.field);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<AuditOptionListRsp>(`/api/v1/audit/model/option/list?${sp}`);
  }

  async fetchModelTrend(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<ModelTrendRsp> {
    const sp = new URLSearchParams(params);
    return this.request<ModelTrendRsp>(`/api/v1/audit/stats/model/trend?${sp}`);
  }

  async fetchRequestRate(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<RequestRateRsp> {
    const sp = new URLSearchParams(params);
    return this.request<RequestRateRsp>(`/api/v1/audit/stats/request/rate?${sp}`);
  }

  async fetchTokenThroughput(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<TokenThroughputRsp> {
    const sp = new URLSearchParams(params);
    return this.request<TokenThroughputRsp>(`/api/v1/audit/stats/token/throughput?${sp}`);
  }

  async fetchTokenRate(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<TokenRateRsp> {
    const sp = new URLSearchParams(params);
    return this.request<TokenRateRsp>(`/api/v1/audit/stats/token/rate?${sp}`);
  }

  async fetchModelUsage(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<ModelUsageRsp> {
    const sp = new URLSearchParams(params);
    return this.request<ModelUsageRsp>(`/api/v1/audit/stats/model/usage?${sp}`);
  }

  async fetchFirstTokenLatency(params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }): Promise<FirstTokenLatencyRsp> {
    const sp = new URLSearchParams(params);
    return this.request<FirstTokenLatencyRsp>(`/api/v1/audit/stats/token/latency?${sp}`);
  }

  // ─── Blocked Words ─────────────────────────────────────────────────────

  async createBlocked(body: CreateBlockedReqBody): Promise<CommonRsp> {
    return this.request<CommonRsp>("/api/v1/block", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async listBlocked(page: number, pageSize: number, query?: string): Promise<ListBlockedRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListBlockedRsp>(`/api/v1/block/list?${params}`);
  }

  async deleteBlocked(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`/api/v1/block?id=${id}`, { method: "DELETE" });
  }

  // ─── Trace (codex hooks) ─────────────────────────────────────────────────────

  async listTraces(
    page: number = 1,
    pageSize: number = 20,
    query?: string
  ): Promise<ListTracesRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (query) params.set("query", query);
    return this.request<ListTracesRsp>(`/api/v1/trace/list?${params}`);
  }

  async getTrace(id: number): Promise<GetTraceRsp> {
    return this.request<GetTraceRsp>(`/api/v1/trace?id=${id}`);
  }

  async listTraceEvents(
    traceId: number,
    page: number = 1,
    pageSize: number = 50
  ): Promise<ListTraceEventsRsp> {
    const params = new URLSearchParams({
      id: String(traceId),
      page: String(page),
      pageSize: String(pageSize),
    });
    return this.request<ListTraceEventsRsp>(`/api/v1/trace/event/list?${params}`);
  }

  async deleteTrace(traceId: number): Promise<DeleteTraceRsp> {
    return this.request<DeleteTraceRsp>(
      `/api/v1/trace?ids=${traceId}`,
      { method: "DELETE" }
    );
  }

  async batchDeleteTraces(ids: number[]): Promise<DeleteTraceRsp> {
    return this.request<DeleteTraceRsp>(
      `/api/v1/trace?ids=${ids.join(",")}`,
      { method: "DELETE" }
    );
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
    return this.request<ListCronJobsRsp>(`/api/v1/cron/list?${sp}`);
  }

  async updateCronJob(body: UpdateCronJobReqBody): Promise<CommonRsp> {
    const payload: Record<string, unknown> = {};
    if (body.enabled !== undefined) payload.enabled = body.enabled;
    if (body.spec !== undefined) payload.spec = body.spec;
    return this.request<CommonRsp>(`/api/v1/cron?name=${encodeURIComponent(body.name)}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
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
    return this.request<ListCronCallAuditsRsp>(`/api/v1/audit/cron/log/list?${sp}`);
  }

  async listCronCallAuditOptions(params: CronCallAuditOptionListReq): Promise<CronCallAuditOptionListRsp> {
    const sp = new URLSearchParams();
    sp.set("field", params.field);
    if (params.keyword) sp.set("keyword", params.keyword);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<CronCallAuditOptionListRsp>(`/api/v1/audit/cron/option/list?${sp}`);
  }

  async getRuntimeMetrics(params: { range: string; since?: number }): Promise<RuntimeMetricsRsp> {
    const sp = new URLSearchParams({ range: params.range });
    if (params.since && params.since > 0) sp.set("since", String(params.since));
    return this.request<RuntimeMetricsRsp>(`/api/v1/metrics/runtime?${sp}`);
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
    if (params.modelIds && params.modelIds.length > 0) sp.set("modelIds", params.modelIds.join(","));
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<DatasetPreviewRsp>(`/api/v1/dataset/preview?${sp}`);
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
    if (params.modelIds && params.modelIds.length > 0) sp.set("modelIds", params.modelIds.join(","));
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    if (params.offset !== undefined) sp.set("offset", String(params.offset));
    return this.request<DatasetFormatPreviewRsp>(`/api/v1/dataset/sample?${sp}`);
  }

  async exportDatasetStream(
    params: {
      minScore?: number;
      modelIds?: string[];
      startTime?: string;
      endTime?: string;
    },
    onEvent: (event: DatasetExportSSEEvent) => void,
    signal?: AbortSignal
  ): Promise<void> {
    const sp = new URLSearchParams();
    if (params.minScore) sp.set("minScore", String(params.minScore));
    if (params.modelIds && params.modelIds.length > 0)
      sp.set("modelIds", params.modelIds.join(","));
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);

    const doFetch = () =>
      fetch(`${API_BASE}/api/v1/dataset/export?${sp}`, {
        headers: { ...this.getHeaders() },
        signal,
      });

    let res = await doFetch();

    if (res.status === 401) {
      const refreshed = await this.tryRefreshToken();
      if (refreshed) {
        res = await doFetch();
        if (!res.ok) throw new ApiError(res.status, await res.text());
      } else {
        this.clearAuthAndPromptLogin();
        throw new ApiError(401, "Authentication required");
      }
    }

    if (!res.ok) {
      throw new ApiError(res.status, await res.text());
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
