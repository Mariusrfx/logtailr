import { type ReactNode, useEffect, useRef, useState } from "react"
import { cn } from "@/lib/utils"

interface StatsCardProps {
  title: string
  value: string | number
  subtitle?: string
  icon: ReactNode
  variant?: "default" | "success" | "warning" | "error"
}

const variantStyles = {
  default: "text-accent",
  success: "text-success",
  warning: "text-warning",
  error: "text-error",
}

export function StatsCard({ title, value, subtitle, icon, variant = "default" }: StatsCardProps) {
  return (
    <div className="bg-surface rounded-lg border border-border p-4 hover:bg-surface-hover transition-colors duration-150">
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm font-medium text-text-secondary">{title}</span>
        <div className={cn("h-5 w-5", variantStyles[variant])}>{icon}</div>
      </div>
      <div className="text-2xl font-bold text-text-primary tabular-nums">
        <AnimatedValue value={value} />
      </div>
      {subtitle && (
        <p className="text-xs text-text-secondary mt-1">{subtitle}</p>
      )}
    </div>
  )
}

const DURATION = 400

function AnimatedValue({ value }: { value: string | number }) {
  const num = typeof value === "number" ? value : parseInt(value.replace(/\D/g, ""), 10)
  const isNumeric = !isNaN(num) && typeof value !== "string" || /^\d[\d,]*$/.test(String(value))

  const [display, setDisplay] = useState(isNumeric ? 0 : value)
  const raf = useRef(0)
  const prevNum = useRef(0)

  useEffect(() => {
    if (!isNumeric) {
      setDisplay(value)
      return
    }

    const from = prevNum.current
    const to = num
    prevNum.current = to

    if (from === to) {
      setDisplay(to.toLocaleString())
      return
    }

    const start = performance.now()
    cancelAnimationFrame(raf.current)

    const step = (now: number) => {
      const t = Math.min((now - start) / DURATION, 1)
      const eased = 1 - Math.pow(1 - t, 3) // ease-out cubic
      const current = Math.round(from + (to - from) * eased)
      setDisplay(current.toLocaleString())
      if (t < 1) raf.current = requestAnimationFrame(step)
    }

    raf.current = requestAnimationFrame(step)
    return () => cancelAnimationFrame(raf.current)
  }, [value, num, isNumeric])

  return <>{display}</>
}
