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
}
