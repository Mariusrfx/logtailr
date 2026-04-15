import { useEffect } from "react"
import { useNavigate } from "react-router-dom"

export function useKeyboardShortcuts() {
  const navigate = useNavigate()

  useEffect(() => {
    function handler(e: KeyboardEvent) {
      // Ignore when typing in inputs
      const tag = (e.target as HTMLElement)?.tagName
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return

      // Ctrl+K / Cmd+K — handled by CommandPalette
      if ((e.metaKey || e.ctrlKey) && e.key === "k") return

      // Single key shortcuts (no modifiers)
      if (e.metaKey || e.ctrlKey || e.altKey) return

      switch (e.key.toLowerCase()) {
        case "d":
          navigate("/")
          break
        case "l":
          navigate("/logs")
          break
        case "s":
          navigate("/sources")
          break
        case "a":
          navigate("/alerts")
          break
        case "escape":
          // Handled by individual components
          break
      }
    }

    window.addEventListener("keydown", handler)
    return () => window.removeEventListener("keydown", handler)
  }, [navigate])
}
