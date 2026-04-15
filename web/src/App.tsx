import { Routes, Route } from "react-router-dom"
import { Layout } from "@/components/layout/Layout"
import { Overview } from "@/components/dashboard/Overview"
import { LogViewer } from "@/components/logs/LogViewer"
import { SourceList } from "@/components/sources/SourceList"
import { AlertsPage } from "@/components/alerts/AlertsPage"
import { ConfigPage } from "@/components/config/ConfigPage"

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Overview />} />
        <Route path="/logs" element={<LogViewer />} />
        <Route path="/sources" element={<SourceList />} />
        <Route path="/alerts" element={<AlertsPage />} />
        <Route path="/config" element={<ConfigPage />} />
      </Route>
    </Routes>
  )
}

export default App
