import { toast } from "sonner";
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
  CommonRsp,
  ListAuditLogsRsp,
  ModelTrendRsp,
  RequestRateRsp,
  TokenThroughputRsp,
  TokenRateRsp,
  ModelUsageRsp,
  Granularity,
} from "./types";
import { BusinessErrorCode } from "./api-errors";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || "";
const AUTH_TOAST_DURATION_MS = 10_000;

export class ApiError extends Error {
  status: number;
  body: string;

  constructor(status: number, body: string) {
    super(`API error ${status}: ${body}`);
    this.name = "ApiError";
    this.status = status;
    this.body = body;
  }
}

class ApiClient {
  private refreshing: Promise<boolean> | null = null;
  private authRetried = false;

  private getHeaders(): HeadersInit {
    const headers: HeadersInit = { "Content-Type": "application/json" };
    if (typeof window !== "undefined") {
      const token = localStorage.getItem("access_token");
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
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

  private async handleAuthFailure<T>(path: string, options?: RequestInit): Promise<T> {
    if (this.authRetried) {
      this.clearAuthAndPromptLogin();
      throw new ApiError(401, "Authentication required");
    }
    this.authRetried = true;
    const refreshed = await this.tryRefreshToken();
    if (refreshed) {
      const retryRes = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers: { ...this.getHeaders(), ...options?.headers },
      });
      if (!retryRes.ok) {
        throw new ApiError(retryRes.status, await retryRes.text());
      }
      return retryRes.json();
    }
    this.clearAuthAndPromptLogin();
    throw new ApiError(401, "Authentication required");
  }

  private clearAuthAndPromptLogin(): void {
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    toast.error("Session expired", {
      description: "Please log in again to continue",
      duration: AUTH_TOAST_DURATION_MS,
      action: {
        label: "Login",
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
    this.authRetried = false;
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
  }): Promise<ListSessionsRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.sort) sp.set("sort", params.sort);
    if (params.sortField) sp.set("sortField", params.sortField);
    if (params.startTime) sp.set("startTime", params.startTime);
    if (params.endTime) sp.set("endTime", params.endTime);
    return this.request<ListSessionsRsp>(`/api/v1/session/list?${sp}`);
  }

  async getSession(sessionId: number): Promise<GetSessionRsp> {
    return this.request<GetSessionRsp>(
      `/api/v1/session?sessionId=${sessionId}`
    );
  }

  async getSessionMetadata(sessionId: number): Promise<GetSessionMetadataRsp> {
    return this.request<GetSessionMetadataRsp>(
      `/api/v1/session/metadata?sessionId=${sessionId}`
    );
  }

  async listSessionMessages(
    sessionId: number,
    page: number = 1,
    pageSize: number = 50
  ): Promise<ListSessionMessagesRsp> {
    return this.request<ListSessionMessagesRsp>(
      `/api/v1/session/message/list?sessionId=${sessionId}&page=${page}&pageSize=${pageSize}`
    );
  }

  async listSessionTools(
    sessionId: number,
    page: number = 1,
    pageSize: number = 20
  ): Promise<ListSessionToolsRsp> {
    return this.request<ListSessionToolsRsp>(
      `/api/v1/session/tool/list?sessionId=${sessionId}&page=${page}&pageSize=${pageSize}`
    );
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

  /**
   * Get shared session metadata (public, no auth).
   */
  async getShareMetadata(shareId: string): Promise<GetShareMetadataRsp> {
    const res = await fetch(
      `${API_BASE}/api/v1/session/share/metadata?id=${encodeURIComponent(shareId)}`,
      {
        method: "GET",
        headers: { "Content-Type": "application/json" },
      }
    );
    if (!res.ok) {
      throw new ApiError(res.status, await res.text());
    }
    return res.json();
  }

  /**
   * List shared session messages with pagination (public, no auth).
   */
  async listShareMessages(
    shareId: string,
    page: number = 1,
    pageSize: number = 50
  ): Promise<ListShareMessagesRsp> {
    const res = await fetch(
      `${API_BASE}/api/v1/session/share/message/list?id=${encodeURIComponent(shareId)}&page=${page}&pageSize=${pageSize}`,
      {
        method: "GET",
        headers: { "Content-Type": "application/json" },
      }
    );
    if (!res.ok) {
      throw new ApiError(res.status, await res.text());
    }
    return res.json();
  }

  /**
   * List shared session tools with pagination (public, no auth).
   */
  async listShareTools(
    shareId: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<ListShareToolsRsp> {
    const res = await fetch(
      `${API_BASE}/api/v1/session/share/tool/list?id=${encodeURIComponent(shareId)}&page=${page}&pageSize=${pageSize}`,
      {
        method: "GET",
        headers: { "Content-Type": "application/json" },
      }
    );
    if (!res.ok) {
      throw new ApiError(res.status, await res.text());
    }
    return res.json();
  }

  // ─── API Keys ──────────────────────────────────────────────────────────────

  async listAPIKeys(
    page: number = 1,
    pageSize: number = 1,
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
    pageSize: number = 1,
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
    pageSize: number = 1,
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
    return this.request<ListAuditLogsRsp>(`/api/v1/audit/log/list?${sp}`);
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
}

export const api = new ApiClient();
