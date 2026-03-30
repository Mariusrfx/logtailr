import { useCallback, useRef, useState } from "react"
import { useWebSocket } from "./useWebSocket"
import type { LogLine } from "@/types"

const MAX_LOGS = 100_000

export interface LogFilters {
  levels: Set<string>
  regex: string
  sources: Set<string>
}

export function useLogs() {
  const [logs, setLogs] = useState<LogLine[]>([])
  const [paused, setPaused] = useState(false)
  const pausedRef = useRef(false)
  const bufferRef = useRef<LogLine[]>([])
  const allSourcesRef = useRef<Set<string>>(new Set())
  const [allSources, setAllSources] = useState<string[]>([])

  const handleMessage = useCallback((data: unknown) => {
    const line = data as LogLine
    if (!line.message && !line.level) return

    if (line.source) {
      if (!allSourcesRef.current.has(line.source)) {
        allSourcesRef.current.add(line.source)
        setAllSources(Array.from(allSourcesRef.current).sort())
      }
    }

    if (pausedRef.current) {
      bufferRef.current.push(line)
      if (bufferRef.current.length > MAX_LOGS) {
        bufferRef.current = bufferRef.current.slice(-MAX_LOGS)
      }
      return
    }

    setLogs((prev) => {
      const next = [...prev, line]
      if (next.length > MAX_LOGS) return next.slice(-MAX_LOGS)
      return next
    })
  }, [])

  const { status } = useWebSocket({
    url: "/ws/logs",
    onMessage: handleMessage,
  })

  const pause = useCallback(() => {
    pausedRef.current = true
    setPaused(true)
  }, [])

  const resume = useCallback(() => {
    pausedRef.current = false
    // Flush buffer
    if (bufferRef.current.length > 0) {
      setLogs((prev) => {
        const merged = [...prev, ...bufferRef.current]
        bufferRef.current = []
        if (merged.length > MAX_LOGS) return merged.slice(-MAX_LOGS)
        return merged
      })
    }
    setPaused(false)
  }, [])

  const clear = useCallback(() => {
    setLogs([])
    bufferRef.current = []
  }, [])

  return { logs, paused, pause, resume, clear, wsStatus: status, allSources }
}

export function filterLogs(logs: LogLine[], filters: LogFilters): LogLine[] {
  let result = logs

  if (filters.levels.size > 0) {
    result = result.filter((l) => filters.levels.has(l.level?.toLowerCase()))
  }

  if (filters.sources.size > 0) {
    result = result.filter((l) => filters.sources.has(l.source))
  }

  if (filters.regex) {
    try {
      const re = new RegExp(filters.regex, "i")
      result = result.filter((l) => re.test(l.message))
    } catch {
      // invalid regex, ignore filter
    }
  }

  return result
}
