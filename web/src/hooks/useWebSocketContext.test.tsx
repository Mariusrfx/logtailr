import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { act, cleanup, render, screen } from "@testing-library/react"
import { WsProvider, useWsStatus } from "./useWebSocketContext"

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  static instances: MockWebSocket[] = []
  url: string
  readyState = MockWebSocket.CONNECTING
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  close() {
    if (this.readyState === MockWebSocket.CLOSED) return
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }
}

function lastInstance(): MockWebSocket {
  return MockWebSocket.instances[MockWebSocket.instances.length - 1]
}

function failLast() {
  act(() => {
    const ws = lastInstance()
    ws.readyState = MockWebSocket.CLOSED
    ws.onclose?.()
  })
}

function openLast() {
  act(() => {
    const ws = lastInstance()
    ws.readyState = MockWebSocket.OPEN
    ws.onopen?.()
  })
}

function StatusProbe() {
  const status = useWsStatus()
  return <span data-testid="status">{status}</span>
}

describe("WsProvider reconnect", () => {
  beforeEach(() => {
    MockWebSocket.instances = []
    vi.stubGlobal("WebSocket", MockWebSocket)
    vi.useFakeTimers()
    vi.spyOn(Math, "random").mockReturnValue(0)
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it("reconnects with exponential backoff (1s, 2s, 4s, ...) with zero jitter", () => {
    render(
      <WsProvider>
        <StatusProbe />
      </WsProvider>
    )

    expect(MockWebSocket.instances).toHaveLength(1)
    expect(screen.getByTestId("status").textContent).toBe("connecting")

    failLast()
    expect(screen.getByTestId("status").textContent).toBe("disconnected")
    expect(MockWebSocket.instances).toHaveLength(1)

    act(() => vi.advanceTimersByTime(999))
    expect(MockWebSocket.instances).toHaveLength(1)
    act(() => vi.advanceTimersByTime(1))
    expect(MockWebSocket.instances).toHaveLength(2)

    failLast()
    act(() => vi.advanceTimersByTime(1999))
    expect(MockWebSocket.instances).toHaveLength(2)
    act(() => vi.advanceTimersByTime(1))
    expect(MockWebSocket.instances).toHaveLength(3)

    failLast()
    act(() => vi.advanceTimersByTime(3999))
    expect(MockWebSocket.instances).toHaveLength(3)
    act(() => vi.advanceTimersByTime(1))
    expect(MockWebSocket.instances).toHaveLength(4)
  })

  it("caps the reconnect delay at 30s", () => {
    render(
      <WsProvider>
        <StatusProbe />
      </WsProvider>
    )

    // Delays: 1s, 2s, 4s, 8s, 16s, then capped at 30s (not 32s).
    for (const delay of [1000, 2000, 4000, 8000, 16000]) {
      failLast()
      act(() => vi.advanceTimersByTime(delay))
    }
    expect(MockWebSocket.instances).toHaveLength(6)

    failLast()
    act(() => vi.advanceTimersByTime(29999))
    expect(MockWebSocket.instances).toHaveLength(6)
    act(() => vi.advanceTimersByTime(1))
    expect(MockWebSocket.instances).toHaveLength(7)
  })

  it("resets the delay after a successful connection", () => {
    render(
      <WsProvider>
        <StatusProbe />
      </WsProvider>
    )

    failLast()
    act(() => vi.advanceTimersByTime(1000))
    failLast()
    act(() => vi.advanceTimersByTime(2000))
    expect(MockWebSocket.instances).toHaveLength(3)

    openLast()
    expect(screen.getByTestId("status").textContent).toBe("connected")

    // After a successful open, the next retry goes back to 1s, not 4s.
    act(() => lastInstance().close())
    act(() => vi.advanceTimersByTime(999))
    expect(MockWebSocket.instances).toHaveLength(3)
    act(() => vi.advanceTimersByTime(1))
    expect(MockWebSocket.instances).toHaveLength(4)
  })

  it("does not reconnect after unmount", () => {
    const { unmount } = render(
      <WsProvider>
        <StatusProbe />
      </WsProvider>
    )

    failLast()
    unmount()

    act(() => vi.advanceTimersByTime(60000))
    expect(MockWebSocket.instances).toHaveLength(1)
  })
})
