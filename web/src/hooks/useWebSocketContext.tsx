import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react"
import type { LogLine } from "@/types"

export type WsStatus = "connected" | "disconnected" | "connecting"

type WsListener = (line: LogLine) => void

interface WsContextValue {
  status: WsStatus
  subscribe: (listener: WsListener) => () => void
}

const WsContext = createContext<WsContextValue>({
  status: "disconnected",
  subscribe: () => () => {},
})

export function useWsStatus(): WsStatus {
  return useContext(WsContext).status
}

export function useWsSubscribe(listener: WsListener) {
  const { subscribe } = useContext(WsContext)
  useEffect(() => subscribe(listener), [subscribe, listener])
}

interface WsProviderProps {
  children: ReactNode
}

export function WsProvider({ children }: WsProviderProps) {
  const [status, setStatus] = useState<WsStatus>("disconnected")
  const listenersRef = useRef<Set<WsListener>>(new Set())
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const mountedRef = useRef(true)

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    setStatus("connecting")
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
    const host = window.location.host
    const ws = new WebSocket(`${protocol}//${host}/ws/logs`)

    ws.onopen = () => {
      if (mountedRef.current) setStatus("connected")
    }

    ws.onmessage = (event) => {
      try {
        const line = JSON.parse(event.data) as LogLine
        for (const fn of listenersRef.current) {
          fn(line)
        }
      } catch {
        // ignore non-JSON
      }
    }

    ws.onclose = () => {
      if (!mountedRef.current) return
      setStatus("disconnected")
      reconnectTimer.current = setTimeout(connect, 3000)
    }

    ws.onerror = () => {
      ws.close()
    }

    wsRef.current = ws
  }, [])

  useEffect(() => {
    mountedRef.current = true
    connect()
    return () => {
      mountedRef.current = false
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [connect])

  const subscribe = useCallback((listener: WsListener) => {
    listenersRef.current.add(listener)
    return () => {
      listenersRef.current.delete(listener)
    }
  }, [])

  return (
    <WsContext.Provider value={{ status, subscribe }}>
      {children}
    </WsContext.Provider>
  )
}
