import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { BrowserRouter } from "react-router-dom"
import { AlertsPage } from "./AlertsPage"

const mockFetch = vi.fn()
global.fetch = mockFetch

function mockAlertEvents(events: unknown[] = [], total = 0) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    json: () => Promise.resolve({ events, total }),
  })
}

function renderPage() {
  return render(
    <BrowserRouter>
      <AlertsPage />
    </BrowserRouter>
  )
}

beforeEach(() => {
  mockFetch.mockReset()
  localStorage.clear()
})

describe("AlertsPage", () => {
  it("shows loading state initially", () => {
    mockFetch.mockReturnValue(new Promise(() => {})) // never resolves
    renderPage()
    expect(screen.getByText("Loading alerts...")).toBeInTheDocument()
  })

  it("shows empty state when no events", async () => {
    mockAlertEvents([], 0)
    renderPage()
    await waitFor(() => {
      expect(screen.getByText("No alert events found")).toBeInTheDocument()
    })
  })

  it("renders alert events", async () => {
    mockAlertEvents([
      {
        ID: "1",
        RuleName: "oom-detect",
        Severity: "critical",
        Message: "Out of memory",
        Source: "api",
        Count: 1,
        FiredAt: "2026-04-15T10:00:00Z",
        AcknowledgedAt: null,
      },
    ], 1)
    renderPage()
    await waitFor(() => {
      expect(screen.getByText("Out of memory")).toBeInTheDocument()
      expect(screen.getByText("oom-detect")).toBeInTheDocument()
      expect(screen.getByText("critical")).toBeInTheDocument()
    })
  })

  it("shows acknowledge button for unacknowledged events", async () => {
    mockAlertEvents([
      {
        ID: "1",
        RuleName: "test",
        Severity: "warning",
        Message: "test alert",
        Source: "",
        Count: 1,
        FiredAt: "2026-04-15T10:00:00Z",
        AcknowledgedAt: null,
      },
    ], 1)
    renderPage()
    await waitFor(() => {
      expect(screen.getByText("Acknowledge")).toBeInTheDocument()
    })
  })

  it("shows Acked for acknowledged events", async () => {
    mockAlertEvents([
      {
        ID: "1",
        RuleName: "test",
        Severity: "warning",
        Message: "test alert",
        Source: "",
        Count: 1,
        FiredAt: "2026-04-15T10:00:00Z",
        AcknowledgedAt: "2026-04-15T10:05:00Z",
      },
    ], 1)
    renderPage()
    await waitFor(() => {
      expect(screen.getByText("Acked")).toBeInTheDocument()
    })
  })

  it("filters by severity", async () => {
    mockAlertEvents([], 0)
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => {
      expect(screen.getByText("No alert events found")).toBeInTheDocument()
    })
    mockAlertEvents([], 0)
    const select = screen.getByDisplayValue("All severities")
    await user.selectOptions(select, "critical")
    await waitFor(() => {
      const url = mockFetch.mock.calls[mockFetch.mock.calls.length - 1][0] as string
      expect(url).toContain("severity=critical")
    })
  })
})
