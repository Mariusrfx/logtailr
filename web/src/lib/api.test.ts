import { describe, it, expect, vi, beforeEach } from "vitest"
import { api } from "./api"

const mockFetch = vi.fn()
global.fetch = mockFetch

beforeEach(() => {
  mockFetch.mockReset()
  localStorage.clear()
})

function mockResponse(data: unknown, status = 200) {
  mockFetch.mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(data),
  })
}

describe("api.getHealth", () => {
  it("calls /health", async () => {
    mockResponse({ status: "healthy" })
    const result = await api.getHealth()
    expect(result).toEqual({ status: "healthy" })
    expect(mockFetch).toHaveBeenCalledWith("/health", expect.objectContaining({
      headers: expect.objectContaining({ "Content-Type": "application/json" }),
    }))
  })
})

describe("api.getAlertEvents", () => {
  it("calls /api/v1/alert-events with query params", async () => {
    mockResponse({ events: [], total: 0 })
    await api.getAlertEvents({ limit: 10, severity: "critical" })
    const url = mockFetch.mock.calls[0][0] as string
    expect(url).toContain("/api/v1/alert-events")
    expect(url).toContain("limit=10")
    expect(url).toContain("severity=critical")
  })

  it("calls without query params when none provided", async () => {
    mockResponse({ events: [], total: 0 })
    await api.getAlertEvents()
    expect(mockFetch.mock.calls[0][0]).toBe("/api/v1/alert-events")
  })
})

describe("api.acknowledgeAlertEvent", () => {
  it("calls POST", async () => {
    mockResponse({ status: "acknowledged" })
    await api.acknowledgeAlertEvent("abc-123")
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/v1/alert-events/abc-123/acknowledge",
      expect.objectContaining({ method: "POST" })
    )
  })
})

describe("api.listSources", () => {
  it("calls /api/v1/sources", async () => {
    mockResponse({ sources: [], total: 0 })
    await api.listSources()
    expect(mockFetch.mock.calls[0][0]).toBe("/api/v1/sources")
  })
})

describe("api.createSource", () => {
  it("sends POST with body", async () => {
    mockResponse({ ID: "1", Name: "test" })
    await api.createSource({ name: "test", type: "file", path: "/var/log/test.log" })
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/v1/sources",
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining('"name":"test"'),
      })
    )
  })
})

describe("api.deleteSource", () => {
  it("sends DELETE", async () => {
    mockResponse(undefined)
    await api.deleteSource("abc-123")
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/v1/sources/abc-123",
      expect.objectContaining({ method: "DELETE" })
    )
  })
})

describe("api.importYaml", () => {
  it("sends raw YAML with correct content-type", async () => {
    mockResponse({ imported: { sources: 1, outputs: 0, alert_rules: 0 } })
    await api.importYaml("sources:\n  - name: test")
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/v1/import/yaml",
      expect.objectContaining({
        method: "POST",
        body: "sources:\n  - name: test",
        headers: expect.objectContaining({ "Content-Type": "application/x-yaml" }),
      })
    )
  })
})

describe("auth token", () => {
  it("includes Authorization header when token is set", async () => {
    localStorage.setItem("logtailr_api_token", "my-token")
    mockResponse({ status: "healthy" })
    await api.getHealth()
    expect(mockFetch).toHaveBeenCalledWith("/health", expect.objectContaining({
      headers: expect.objectContaining({ Authorization: "Bearer my-token" }),
    }))
  })

  it("omits Authorization header when no token", async () => {
    mockResponse({ status: "healthy" })
    await api.getHealth()
    const headers = mockFetch.mock.calls[0][1].headers
    expect(headers.Authorization).toBeUndefined()
  })
})

describe("error handling", () => {
  it("throws on non-ok response with error message", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: () => Promise.resolve({ error: "database not configured" }),
    })
    await expect(api.getHealth()).rejects.toThrow("database not configured")
  })

  it("throws with status code when no error body", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.reject(new Error("no json")),
    })
    await expect(api.getHealth()).rejects.toThrow("HTTP 500")
  })
})
