import { useEffect } from "react"
import { useLocation } from "react-router-dom"
import type { HealthSummary } from "@/types"

const routeTitles: Record<string, string> = {
  "/": "Dashboard",
  "/logs": "Logs",
  "/sources": "Sources",
  "/config": "Config",
}

export function useDynamicTitle(health: HealthSummary | null) {
  const location = useLocation()

  useEffect(() => {
    const page = routeTitles[location.pathname] || "Logtailr"
    const errorCount = health?.sources.failed ?? 0

    if (errorCount > 0) {
      document.title = `(${errorCount} failed) ${page} - Logtailr`
    } else {
      document.title = `${page} - Logtailr`
    }
  }, [location.pathname, health])
}
