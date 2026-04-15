const BASE = ""

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }

  const token = localStorage.getItem("logtailr_api_token")
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  // Allow callers to override headers (e.g. Content-Type for YAML import)
  if (options?.headers) {
    Object.assign(headers, options.headers)
  }

  const res = await fetch(`${BASE}${path}`, { ...options, headers })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  getHealth: () => request<Record<string, unknown>>("/health"),
  getHealthSources: () => request<Record<string, unknown>>("/health/sources"),
  getAlerts: () => request<Record<string, unknown>>("/alerts"),
  getAlertRules: () => request<Record<string, unknown>>("/alerts/rules"),

  getAlertEvents: (params?: Record<string, string | number>) => {
    const qs = new URLSearchParams()
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined && v !== "") qs.set(k, String(v))
      }
    }
    const query = qs.toString()
    return request<{ events: import("@/types").AlertEventRow[]; total: number }>(
      `/api/v1/alert-events${query ? `?${query}` : ""}`
    )
  },

  acknowledgeAlertEvent: (id: string) =>
    request<{ status: string }>(`/api/v1/alert-events/${id}/acknowledge`, {
      method: "POST",
    }),

  // Sources CRUD
  listSources: () =>
    request<{ sources: import("@/types").SourceRow[]; total: number }>("/api/v1/sources"),
  createSource: (data: import("@/types").SourceRequest) =>
    request<import("@/types").SourceRow>("/api/v1/sources", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateSource: (id: string, data: import("@/types").SourceRequest) =>
    request<import("@/types").SourceRow>(`/api/v1/sources/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteSource: (id: string) =>
    request<void>(`/api/v1/sources/${id}`, { method: "DELETE" }),

  // Outputs CRUD
  listOutputs: () =>
    request<{ outputs: import("@/types").OutputRow[]; total: number }>("/api/v1/outputs"),
  createOutput: (data: import("@/types").OutputRequest) =>
    request<import("@/types").OutputRow>("/api/v1/outputs", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateOutput: (id: string, data: import("@/types").OutputRequest) =>
    request<import("@/types").OutputRow>(`/api/v1/outputs/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteOutput: (id: string) =>
    request<void>(`/api/v1/outputs/${id}`, { method: "DELETE" }),

  // Alert Rules CRUD
  listAlertRules: () =>
    request<{ rules: import("@/types").AlertRuleRow[]; total: number }>("/api/v1/alert-rules"),
  createAlertRule: (data: import("@/types").AlertRuleRequest) =>
    request<import("@/types").AlertRuleRow>("/api/v1/alert-rules", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateAlertRule: (id: string, data: import("@/types").AlertRuleRequest) =>
    request<import("@/types").AlertRuleRow>(`/api/v1/alert-rules/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteAlertRule: (id: string) =>
    request<void>(`/api/v1/alert-rules/${id}`, { method: "DELETE" }),

  // Settings
  getSetting: (key: string) =>
    request<import("@/types").SettingValue>(`/api/v1/settings/${key}`),
  setSetting: (key: string, value: unknown) =>
    request<import("@/types").SettingValue>(`/api/v1/settings/${key}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),

  // Import YAML
  importYaml: (yaml: string) =>
    request<import("@/types").ImportResult>("/api/v1/import/yaml", {
      method: "POST",
      body: yaml,
      headers: { "Content-Type": "application/x-yaml" },
    }),
}
