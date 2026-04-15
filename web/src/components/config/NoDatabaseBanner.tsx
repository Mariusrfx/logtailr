import { Database } from "lucide-react"

export function isNoDatabaseError(error: string): boolean {
  return error.toLowerCase().includes("database not configured") || error.includes("503")
}

export function NoDatabaseBanner() {
  return (
    <div className="p-8 text-center">
      <Database className="h-8 w-8 text-text-secondary mx-auto mb-3 opacity-50" />
      <p className="text-sm text-text-secondary">Database not configured</p>
    </div>
  )
}
