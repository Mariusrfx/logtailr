import { describe, it, expect } from "vitest"
import { formatTimestamp, formatRelativeTime } from "./utils"

describe("formatTimestamp", () => {
  it("formats ISO timestamp to HH:MM:SS", () => {
    const result = formatTimestamp("2026-04-15T14:30:45Z")
    expect(result).toMatch(/\d{2}:\d{2}:\d{2}/)
  })

  it("handles invalid timestamp", () => {
    const result = formatTimestamp("not-a-date")
    expect(result).toBe("Invalid Date")
  })
})

describe("formatRelativeTime", () => {
  it("shows seconds for recent times", () => {
    const now = new Date()
    const result = formatRelativeTime(now.toISOString())
    expect(result).toMatch(/\d+s ago/)
  })

  it("shows minutes", () => {
    const fiveMinAgo = new Date(Date.now() - 5 * 60 * 1000)
    const result = formatRelativeTime(fiveMinAgo.toISOString())
    expect(result).toBe("5m ago")
  })

  it("shows hours", () => {
    const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000)
    const result = formatRelativeTime(twoHoursAgo.toISOString())
    expect(result).toBe("2h ago")
  })

  it("shows days", () => {
    const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000)
    const result = formatRelativeTime(threeDaysAgo.toISOString())
    expect(result).toBe("3d ago")
  })
})
