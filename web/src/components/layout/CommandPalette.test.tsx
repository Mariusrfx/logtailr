import { describe, it, expect, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { BrowserRouter } from "react-router-dom"
import { CommandPalette } from "./CommandPalette"

function renderPalette() {
  return render(
    <BrowserRouter>
      <CommandPalette />
    </BrowserRouter>
  )
}

describe("CommandPalette", () => {
  it("is hidden by default", () => {
    renderPalette()
    expect(screen.queryByPlaceholderText("Type a command...")).not.toBeInTheDocument()
  })

  it("opens with Ctrl+K", async () => {
    const user = userEvent.setup()
    renderPalette()
    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() => {
      expect(screen.getByPlaceholderText("Type a command...")).toBeInTheDocument()
    })
  })

  it("shows all commands when open", async () => {
    const user = userEvent.setup()
    renderPalette()
    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() => {
      expect(screen.getByText("Dashboard")).toBeInTheDocument()
      expect(screen.getByText("Logs")).toBeInTheDocument()
      expect(screen.getByText("Sources")).toBeInTheDocument()
      expect(screen.getByText("Alerts")).toBeInTheDocument()
      expect(screen.getByText("Configuration")).toBeInTheDocument()
    })
  })

  it("filters commands by search query", async () => {
    const user = userEvent.setup()
    renderPalette()
    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() => {
      expect(screen.getByPlaceholderText("Type a command...")).toBeInTheDocument()
    })
    await user.type(screen.getByPlaceholderText("Type a command..."), "log")
    await waitFor(() => {
      expect(screen.getByText("Logs")).toBeInTheDocument()
      expect(screen.queryByText("Dashboard")).not.toBeInTheDocument()
    })
  })

  it("shows no results for non-matching query", async () => {
    const user = userEvent.setup()
    renderPalette()
    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() => {
      expect(screen.getByPlaceholderText("Type a command...")).toBeInTheDocument()
    })
    await user.type(screen.getByPlaceholderText("Type a command..."), "zzzzz")
    await waitFor(() => {
      expect(screen.getByText("No results")).toBeInTheDocument()
    })
  })

  it("closes with Escape", async () => {
    const user = userEvent.setup()
    renderPalette()
    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() => {
      expect(screen.getByPlaceholderText("Type a command...")).toBeInTheDocument()
    })
    await user.keyboard("{Escape}")
    await waitFor(() => {
      expect(screen.queryByPlaceholderText("Type a command...")).not.toBeInTheDocument()
    })
  })

  it("toggles with Ctrl+K", async () => {
    const user = userEvent.setup()
    renderPalette()
    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() => {
      expect(screen.getByPlaceholderText("Type a command...")).toBeInTheDocument()
    })
    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() => {
      expect(screen.queryByPlaceholderText("Type a command...")).not.toBeInTheDocument()
    })
  })
})
