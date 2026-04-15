import { useCallback, useMemo, useRef, useState } from "react"
import { useVirtualizer } from "@tanstack/react-virtual"
import { ArrowLeft, CheckCircle, AlertTriangle, XCircle, StopCircle } from "lucide-react"
import type { SourceHealth, LogLine } from "@/types"
import { cn, formatRelativeTime } from "@/lib/utils"
import { useLogs, filterLogs } from "@/hooks/useLogs"
import { LogRow } from "@/components/logs/LogRow"

interface SourceDetailProps {
  source: SourceHealth
  onBack: () => void
}

const statusConfig = {
  healthy: { icon: CheckCircle, color: "text-success", bg: "bg-success/10", label: "Healthy" },
  degraded: { icon: AlertTriangle, color: "text-warning", bg: "bg-warning/10", label: "Degraded" },
  failed: { icon: XCircle, color: "text-error", bg: "bg-error/10", label: "Failed" },
  stopped: { icon: StopCircle, color: "text-text-secondary", bg: "bg-surface-hover", label: "Stopped" },
}

export function SourceDetail({ source, onBack }: SourceDetailProps) {
  const config = statusConfig[source.status] || statusConfig.stopped
  const StatusIcon = config.icon

  return (
    <div className="space-y-6">
      {/* Back button + header */}
      <div className="flex items-center gap-3">
        <button
          onClick={onBack}
          className="p-2 rounded-md text-text-secondary hover:bg-surface-hover hover:text-text-primary transition-colors"
        >
          <ArrowLeft className="h-5 w-5" />
        </button>
        <div>
          <h2 className="text-xl font-bold text-text-primary">{source.name}</h2>
          <div className="flex items-center gap-2 mt-1">
            <div className={cn("flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium", config.bg, config.color)}>
              <StatusIcon className="h-3.5 w-3.5" />
              {config.label}
            </div>
          </div>
        </div>
      </div>

      {/* Info grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <InfoCard label="Status" value={config.label}>
          <StatusIcon className={cn("h-5 w-5", config.color)} />
        </InfoCard>
        <InfoCard label="Error Count" value={String(source.error_count)}>
          {source.error_count > 0 && (
            <span className="text-xs text-error font-medium">{source.error_count} errors</span>
          )}
        </InfoCard>
        <InfoCard label="Uptime" value={source.uptime || "N/A"} />
        <InfoCard label="Last Update" value={formatRelativeTime(source.last_update)} />
      </div>

      {/* Last error */}
      {source.last_error && (
        <div className="bg-surface rounded-lg border border-border p-4">
          <h3 className="text-sm font-medium text-text-secondary mb-2">Last Error</h3>
          <div className="p-3 rounded bg-error/5 border border-error/10">
            <p className="text-sm text-error font-mono break-words">{source.last_error}</p>
          </div>
        </div>
      )}

      {/* Inline recent logs */}
      <SourceLogs sourceName={source.name} />
    </div>
  )
}

function InfoCard({ label, value, children }: { label: string; value: string; children?: React.ReactNode }) {
  return (
    <div className="bg-surface rounded-lg border border-border p-4">
      <p className="text-xs text-text-secondary mb-1">{label}</p>
      <p className="text-lg font-semibold text-text-primary">{value}</p>
      {children}
    </div>
  )
}

const MAX_SOURCE_LOGS = 200
const ROW_HEIGHT = 28

function SourceLogs({ sourceName }: { sourceName: string }) {
  const { logs } = useLogs()
  const [selectedLog, setSelectedLog] = useState<LogLine | null>(null)

  const sourceFilter = useMemo(
    () => ({ levels: new Set<string>(), regex: "", sources: new Set([sourceName]) }),
    [sourceName]
  )
  const filtered = filterLogs(logs, sourceFilter).slice(-MAX_SOURCE_LOGS)

  const parentRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  })

  const handleClick = useCallback((log: LogLine) => {
    setSelectedLog((prev) => (prev === log ? null : log))
  }, [])

  return (
    <div className="bg-surface rounded-lg border border-border overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 border-b border-border">
        <h3 className="text-sm font-medium text-text-secondary">
          Recent Logs
          {filtered.length > 0 && (
            <span className="ml-2 text-xs text-text-secondary">({filtered.length})</span>
          )}
        </h3>
        <a href="/logs" className="text-xs text-accent hover:underline">
          Open full viewer
        </a>
      </div>

      {filtered.length === 0 ? (
        <div className="p-6 text-center text-sm text-text-secondary">
          No logs received for this source yet
        </div>
      ) : (
        <>
          <div ref={parentRef} className="overflow-auto" style={{ maxHeight: 400 }}>
            <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
              {virtualizer.getVirtualItems().map((vRow) => {
                const log = filtered[vRow.index]
                return (
                  <div
                    key={vRow.index}
                    style={{
                      position: "absolute",
                      top: 0,
                      left: 0,
                      width: "100%",
                      height: vRow.size,
                      transform: `translateY(${vRow.start}px)`,
                    }}
                  >
                    <LogRow log={log} onClick={() => handleClick(log)} />
                  </div>
                )
              })}
            </div>
          </div>

          {/* Minimal detail for selected log */}
          {selectedLog && (
            <div className="border-t border-border p-4 bg-surface-hover">
              <div className="flex items-center justify-between mb-2">
                <span className="text-xs font-medium text-text-secondary">Log Detail</span>
                <button
                  onClick={() => setSelectedLog(null)}
                  className="text-xs text-text-secondary hover:text-text-primary"
                >
                  Close
                </button>
              </div>
              <pre className="text-xs text-text-primary font-mono whitespace-pre-wrap break-words">
                {JSON.stringify(selectedLog, null, 2)}
              </pre>
            </div>
          )}
        </>
      )}
    </div>
  )
}
