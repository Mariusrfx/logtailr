import { describe, it, expect } from "vitest"
import { filterLogs, type LogFilters } from "./useLogs"
import type { LogLine } from "@/types"

const makeLine = (overrides: Partial<LogLine> = {}): LogLine => ({
  timestamp: "2026-04-15T10:00:00Z",
  level: "info",
  message: "test message",
  source: "app.log",
  ...overrides,
})

const noFilters: LogFilters = { levels: new Set(), regex: "", sources: new Set() }

describe("filterLogs", () => {
  const logs: LogLine[] = [
    makeLine({ level: "debug", message: "cache hit", source: "api" }),
    makeLine({ level: "info", message: "request processed", source: "api" }),
    makeLine({ level: "warn", message: "high latency", source: "nginx" }),
    makeLine({ level: "error", message: "connection refused", source: "api" }),
    makeLine({ level: "fatal", message: "out of memory", source: "worker" }),
  ]

  it("returns all logs when no filters active", () => {
    expect(filterLogs(logs, noFilters)).toHaveLength(5)
  })

  it("filters by single level", () => {
    const filters: LogFilters = { levels: new Set(["error"]), regex: "", sources: new Set() }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(1)
    expect(result[0].level).toBe("error")
  })

  it("filters by multiple levels", () => {
    const filters: LogFilters = { levels: new Set(["error", "fatal"]), regex: "", sources: new Set() }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(2)
    expect(result.map((l) => l.level)).toEqual(["error", "fatal"])
  })

  it("filters by regex", () => {
    const filters: LogFilters = { levels: new Set(), regex: "connection|memory", sources: new Set() }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(2)
    expect(result[0].message).toBe("connection refused")
    expect(result[1].message).toBe("out of memory")
  })

  it("regex is case insensitive", () => {
    const filters: LogFilters = { levels: new Set(), regex: "CACHE", sources: new Set() }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(1)
  })

  it("ignores invalid regex", () => {
    const filters: LogFilters = { levels: new Set(), regex: "[invalid(", sources: new Set() }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(5)
  })

  it("filters by single source", () => {
    const filters: LogFilters = { levels: new Set(), regex: "", sources: new Set(["api"]) }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(3)
    expect(result.every((l) => l.source === "api")).toBe(true)
  })

  it("filters by multiple sources", () => {
    const filters: LogFilters = { levels: new Set(), regex: "", sources: new Set(["nginx", "worker"]) }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(2)
  })

  it("combines level + source + regex filters", () => {
    const filters: LogFilters = {
      levels: new Set(["error", "fatal"]),
      regex: "connection",
      sources: new Set(["api"]),
    }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(1)
    expect(result[0].message).toBe("connection refused")
  })

  it("returns empty when no logs match", () => {
    const filters: LogFilters = { levels: new Set(["fatal"]), regex: "nonexistent", sources: new Set() }
    const result = filterLogs(logs, filters)
    expect(result).toHaveLength(0)
  })

  it("handles empty log array", () => {
    expect(filterLogs([], noFilters)).toHaveLength(0)
  })
})
