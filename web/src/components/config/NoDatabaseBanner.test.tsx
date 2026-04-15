import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { NoDatabaseBanner, isNoDatabaseError } from "./NoDatabaseBanner"

describe("isNoDatabaseError", () => {
  it("detects 'database not configured' message", () => {
    expect(isNoDatabaseError("database not configured (use --db-url)")).toBe(true)
  })

  it("detects case-insensitive", () => {
    expect(isNoDatabaseError("Database Not Configured")).toBe(true)
  })

  it("detects 503 status", () => {
    expect(isNoDatabaseError("HTTP 503")).toBe(true)
  })

  it("returns false for other errors", () => {
    expect(isNoDatabaseError("connection refused")).toBe(false)
  })
})

describe("NoDatabaseBanner", () => {
  it("renders message", () => {
    render(<NoDatabaseBanner />)
    expect(screen.getByText("Database not configured")).toBeInTheDocument()
  })
})
