import { Routes, Route } from "react-router-dom"
import { Layout } from "@/components/layout/Layout"
import { Overview } from "@/components/dashboard/Overview"
import { LogViewer } from "@/components/logs/LogViewer"
import { SourceList } from "@/components/sources/SourceList"

function Placeholder({ title }: { title: string }) {
  return (
    <div className="flex items-center justify-center h-full min-h-[400px]">
      <div className="text-center">
        <h2 className="text-2xl font-bold text-text-primary">{title}</h2>
        <p className="mt-2 text-text-secondary">Coming next</p>
      </div>
    </div>
  )
}

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Overview />} />
        <Route path="/logs" element={<LogViewer />} />
        <Route path="/sources" element={<SourceList />} />
        <Route path="/config" element={<Placeholder title="Config" />} />
      </Route>
    </Routes>
  )
}

export default App
