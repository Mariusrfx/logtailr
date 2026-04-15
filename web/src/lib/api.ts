const BASE = ""

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }

  const token = localStorage.getItem("logtailr_api_token")
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
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
}
