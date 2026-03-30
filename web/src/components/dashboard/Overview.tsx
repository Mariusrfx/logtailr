import { useCallback, useEffect, useRef, useState } from "react"
import { Activity, AlertTriangle, Clock, Server } from "lucide-react"
import { StatsCard } from "./StatsCard"
import { SourceHealthCard } from "./SourceHealthCard"
import { RecentErrors } from "./RecentErrors"
import { useHealth } from "@/hooks/useHealth"
import { useWebSocket } from "@/hooks/useWebSocket"
import type { AlertEvent, SourceHealth } from "@/types"

export function Overview() {
  const { health } = useHealth(3000)
  const [sources, setSources] = useState<SourceHealth[]>([])
  const [alerts, setAlerts] = useState<AlertEvent[]>([])
  const [logCount, setLogCount] = useState(0)
  const logCountRef = useRef(0)

  // Fetch sources
  useEffect(() => {
    const fetchSources = async () => {
      try {
        const res = await fetch("/health/sources")
        if (!res.ok) return
        const data = await res.json()
        setSources(data.sources || [])
      } catch {
        // ignore
      }
    }
    fetchSources()
    const id = setInterval(fetchSources, 5000)
    return () => clearInterval(id)
  }, [])

  // Fetch alerts
  useEffect(() => {
    const fetchAlerts = async () => {
      try {
        const res = await fetch("/alerts")
        if (!res.ok) return
        const data = await res.json()
        setAlerts((data.alerts || []).slice(-10))
      } catch {
        // ignore
      }
    }
    fetchAlerts()
    const id = setInterval(fetchAlerts, 5000)
    return () => clearInterval(id)
  }, [])

  // Count logs from WebSocket
  const handleWsMessage = useCallback(() => {
    logCountRef.current++
    setLogCount(logCountRef.current)
  }, [])

  useWebSocket({
    url: "/ws/logs",
    onMessage: handleWsMessage,
  })

  const errorCount = sources.reduce((sum, s) => sum + s.error_count, 0)

  return (
    <div className="space-y-6">
      {/* Stats cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard
          title="Total Logs"
          value={logCount.toLocaleString()}
          subtitle="Since page load"
          icon={<Activity className="h-5 w-5" />}
        />
        <StatsCard
          title="Errors"
          value={errorCount}
          subtitle="Across all sources"
          icon={<AlertTriangle className="h-5 w-5" />}
          variant={errorCount > 0 ? "error" : "default"}
        />
        <StatsCard
          title="Sources"
          value={health ? `${health.sources.healthy}/${health.sources.total}` : "-"}
          subtitle="Healthy"
          icon={<Server className="h-5 w-5" />}
          variant={
            health && health.sources.failed > 0
              ? "error"
              : health && health.sources.degraded > 0
              ? "warning"
              : "success"
          }
        />
        <StatsCard
          title="Uptime"
          value={health?.uptime ?? "-"}
          subtitle="Since start"
          icon={<Clock className="h-5 w-5" />}
        />
      </div>

      {/* Source health grid */}
      {sources.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-text-secondary mb-3">Source Health</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
            {sources.map((source) => (
              <SourceHealthCard key={source.name} source={source} />
            ))}
          </div>
        </div>
      )}

      {/* Recent errors */}
      <RecentErrors events={alerts} />
    </div>
  )
}
