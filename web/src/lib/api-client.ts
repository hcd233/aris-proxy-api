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
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || "";

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

  private async request<T>(
    path: string,
    options?: RequestInit
  ): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: { ...this.getHeaders(), ...options?.headers },
    });

    if (res.status === 401) {
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
      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/web/login";
      throw new ApiError(401, "Authentication required");
    }

    if (!res.ok) {
      throw new ApiError(res.status, await res.text());
    }

    return res.json();
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

  async listSessions(
    page: number = 1,
    pageSize: number = 20
  ): Promise<ListSessionsRsp> {
    return this.request<ListSessionsRsp>(
      `/api/v1/session/list?page=${page}&pageSize=${pageSize}`
    );
  }

  async getSession(sessionId: number): Promise<GetSessionRsp> {
    return this.request<GetSessionRsp>(
      `/api/v1/session/?sessionId=${sessionId}`
    );
  }

  // ─── API Keys ──────────────────────────────────────────────────────────────

  async listAPIKeys(): Promise<ListAPIKeysRsp> {
    return this.request<ListAPIKeysRsp>("/api/v1/apikey/");
  }

  async createAPIKey(
    body: CreateAPIKeyReqBody
  ): Promise<CreateAPIKeyRsp> {
    return this.request<CreateAPIKeyRsp>("/api/v1/apikey/", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async deleteAPIKey(id: number): Promise<void> {
    await this.request(`/api/v1/apikey/${id}`, {
      method: "DELETE",
    });
  }

  // ─── Endpoints (admin) ─────────────────────────────────────────────────────

  async listEndpoints(): Promise<ListEndpointsRsp> {
    return this.request<ListEndpointsRsp>("/api/v1/endpoint/");
  }

  async createEndpoint(
    body: CreateEndpointReqBody
  ): Promise<ListEndpointsRsp> {
    return this.request<ListEndpointsRsp>("/api/v1/endpoint/", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async updateEndpoint(
    id: number,
    body: UpdateEndpointReqBody
  ): Promise<ListEndpointsRsp> {
    return this.request<ListEndpointsRsp>(`/api/v1/endpoint/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async deleteEndpoint(id: number): Promise<void> {
    await this.request(`/api/v1/endpoint/${id}`, {
      method: "DELETE",
    });
  }

  // ─── Models (admin) ────────────────────────────────────────────────────────

  async listModels(): Promise<ListModelsRsp> {
    return this.request<ListModelsRsp>("/api/v1/model/");
  }

  async createModel(body: CreateModelReqBody): Promise<ListModelsRsp> {
    return this.request<ListModelsRsp>("/api/v1/model/", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async updateModel(
    id: number,
    body: UpdateModelReqBody
  ): Promise<ListModelsRsp> {
    return this.request<ListModelsRsp>(`/api/v1/model/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }

  async deleteModel(id: number): Promise<void> {
    await this.request(`/api/v1/model/${id}`, {
      method: "DELETE",
    });
  }
}

export const api = new ApiClient();
