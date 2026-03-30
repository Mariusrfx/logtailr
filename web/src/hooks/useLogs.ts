import { useCallback, useRef, useState } from "react"
import { useWsSubscribe } from "./useWebSocketContext"
import type { LogLine } from "@/types"

const MAX_LOGS = 100_000
const RENDER_THROTTLE_MS = 100

export interface LogFilters {
  levels: Set<string>
  regex: string
  sources: Set<string>
}

export function useLogs() {
  const [logs, setLogs] = useState<LogLine[]>([])
  const [paused, setPaused] = useState(false)
  const pausedRef = useRef(false)

  // Mutable buffer — no re-renders on each push
  const bufferRef = useRef<LogLine[]>([])
  const pauseBufferRef = useRef<LogLine[]>([])
  const flushScheduled = useRef(false)

  const allSourcesRef = useRef<Set<string>>(new Set())
  const [allSources, setAllSources] = useState<string[]>([])

  // Flush buffer to state (throttled)
  const scheduleFlush = useCallback(() => {
    if (flushScheduled.current) return
    flushScheduled.current = true

    setTimeout(() => {
      flushScheduled.current = false
      const batch = bufferRef.current
      if (batch.length === 0) return
      bufferRef.current = []

      setLogs((prev) => {
        const merged = prev.length + batch.length > MAX_LOGS
          ? [...prev.slice(-(MAX_LOGS - batch.length)), ...batch]
          : [...prev, ...batch]
        return merged
      })
    }, RENDER_THROTTLE_MS)
  }, [])

  const handleLine = useCallback((line: LogLine) => {
    if (!line.message && !line.level) return

    if (line.source && !allSourcesRef.current.has(line.source)) {
      allSourcesRef.current.add(line.source)
      setAllSources(Array.from(allSourcesRef.current).sort())
    }

    if (pausedRef.current) {
      pauseBufferRef.current.push(line)
      if (pauseBufferRef.current.length > MAX_LOGS) {
        pauseBufferRef.current = pauseBufferRef.current.slice(-MAX_LOGS)
      }
      return
    }

    bufferRef.current.push(line)
    scheduleFlush()
  }, [scheduleFlush])

  useWsSubscribe(handleLine)

  const pause = useCallback(() => {
    pausedRef.current = true
    setPaused(true)
  }, [])

  const resume = useCallback(() => {
    pausedRef.current = false
    if (pauseBufferRef.current.length > 0) {
      bufferRef.current.push(...pauseBufferRef.current)
      pauseBufferRef.current = []
      scheduleFlush()
    }
    setPaused(false)
  }, [scheduleFlush])

  const clear = useCallback(() => {
    setLogs([])
    bufferRef.current = []
    pauseBufferRef.current = []
  }, [])

  return { logs, paused, pause, resume, clear, allSources }
}

export function filterLogs(logs: LogLine[], filters: LogFilters): LogLine[] {
  // Fast path: no filters active
  if (filters.levels.size === 0 && filters.sources.size === 0 && !filters.regex) {
    return logs
  }

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
