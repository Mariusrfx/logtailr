import { useCallback, useEffect, useState } from "react"
import { SourceCard } from "./SourceCard"
import { SourceDetail } from "./SourceDetail"
import type { SourceHealth } from "@/types"
import { cn } from "@/lib/utils"

type StatusFilter = "all" | "healthy" | "degraded" | "failed" | "stopped"

const filterOptions: { value: StatusFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "healthy", label: "Healthy" },
  { value: "degraded", label: "Degraded" },
  { value: "failed", label: "Failed" },
  { value: "stopped", label: "Stopped" },
]

export function SourceList() {
  const [sources, setSources] = useState<SourceHealth[]>([])
  const [filter, setFilter] = useState<StatusFilter>("all")
  const [selected, setSelected] = useState<SourceHealth | null>(null)

  const fetchSources = useCallback(async () => {
    try {
      const res = await fetch("/health/sources")
      if (!res.ok) return
      const data = await res.json()
      setSources(data.sources || [])
      // Update selected if still viewing detail
      if (selected) {
        const updated = (data.sources || []).find((s: SourceHealth) => s.name === selected.name)
        if (updated) setSelected(updated)
      }
    } catch {
      // ignore
    }
  }, [selected])

  useEffect(() => {
    fetchSources()
    const id = setInterval(fetchSources, 3000)
    return () => clearInterval(id)
  }, [fetchSources])

  if (selected) {
    return <SourceDetail source={selected} onBack={() => setSelected(null)} />
  }

  const filtered = filter === "all" ? sources : sources.filter((s) => s.status === filter)

  const counts = {
    all: sources.length,
    healthy: sources.filter((s) => s.status === "healthy").length,
    degraded: sources.filter((s) => s.status === "degraded").length,
    failed: sources.filter((s) => s.status === "failed").length,
    stopped: sources.filter((s) => s.status === "stopped").length,
  }

  return (
    <div className="space-y-4">
      {/* Filter bar */}
      <div className="flex items-center gap-2 flex-wrap">
        {filterOptions.map((opt) => (
          <button
            key={opt.value}
            onClick={() => setFilter(opt.value)}
            className={cn(
              "px-3 py-1.5 rounded-md text-sm font-medium transition-colors duration-150",
              filter === opt.value
                ? "bg-accent/10 text-accent"
                : "text-text-secondary hover:bg-surface-hover hover:text-text-primary"
            )}
          >
            {opt.label}
            {counts[opt.value] > 0 && (
              <span className="ml-1.5 text-xs opacity-70">({counts[opt.value]})</span>
            )}
          </button>
        ))}
      </div>

      {/* Source grid */}
      {filtered.length > 0 ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {filtered.map((source) => (
            <SourceCard
              key={source.name}
              source={source}
              onClick={() => setSelected(source)}
            />
          ))}
        </div>
      ) : (
        <div className="flex items-center justify-center min-h-[200px]">
          <p className="text-sm text-text-secondary">
            {sources.length === 0
              ? "No sources configured"
              : "No sources match the selected filter"}
          </p>
        </div>
      )}
    </div>
  )
}
