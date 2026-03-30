import { useCallback, useEffect, useState } from "react"
import type { HealthSummary } from "@/types"

export function useHealth(intervalMs = 5000) {
  const [health, setHealth] = useState<HealthSummary | null>(null)
  const [error, setError] = useState<string | null>(null)

  const fetchHealth = useCallback(async () => {
    try {
      const res = await fetch("/health")
      if (!res.ok) {
        setError(`HTTP ${res.status}`)
        return
      }
      const data = await res.json()
      setHealth(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unknown error")
    }
  }, [])

  useEffect(() => {
    void fetchHealth()
    const id = setInterval(() => void fetchHealth(), intervalMs)
    return () => clearInterval(id)
  }, [fetchHealth, intervalMs])

  return { health, error }
}
