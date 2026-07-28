import { useState, type ReactNode } from "react"
import { MonitorCog } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { DownloadsSection } from "@/features/downloads/downloads-section"
import { OrganizationSection } from "@/features/organization/organization-section"
import { SchedulerSection } from "@/features/scheduler/scheduler-section"
import { SiteProfiles } from "@/features/settings/site-profiles"
import { clearPrivateData, setGlobalBandwidthLimit, type HealthStatus } from "@/lib/backend"
import { type AppSection, useUIStore } from "@/stores/ui-store"

interface SectionContentProps {
  section: AppSection
  health: HealthStatus | null
  hasBackendError: boolean
  addDialogOpen: boolean
  onAddDialogOpenChange: (open: boolean) => void
}

export function SectionContent({ section, health, hasBackendError, addDialogOpen, onAddDialogOpenChange }: SectionContentProps) {
  switch (section) {
    case "categories":
      return <OrganizationSection />
    case "scheduler":
      return <SchedulerSection />
    case "settings":
      return <SettingsSection />
    case "downloads":
      return <DownloadsSection health={health} hasBackendError={hasBackendError} addDialogOpen={addDialogOpen} onAddDialogOpenChange={onAddDialogOpenChange} />
  }
}

function SettingsSection() {
  const density = useUIStore((state) => state.density)
  const setDensity = useUIStore((state) => state.setDensity)
  const [globalLimitMiB, setGlobalLimitMiB] = useState("0")
  const [limitStatus, setLimitStatus] = useState<string | null>(null)
	const [privacyStatus,setPrivacyStatus]=useState<string|null>(null)

  const applyGlobalLimit = async () => {
    const value = Number(globalLimitMiB)
    if (!Number.isFinite(value) || value < 0) { setLimitStatus("Enter a non-negative limit."); return }
    try {
      await setGlobalBandwidthLimit(Math.round(value * 1024 * 1024))
      setLimitStatus(value === 0 ? "Global limit disabled." : `Global limit set to ${value} MiB/s.`)
    } catch { setLimitStatus("Could not apply the global limit.") }
  }

  return (
    <Panel title="Interface settings" description="Foundation preferences are kept in UI state for now." badge="Local">
      <div className="p-6">
        <div className="flex max-w-2xl items-center justify-between rounded-xl border border-border bg-surface p-4">
          <div className="flex items-center gap-4">
            <div className="grid size-10 place-items-center rounded-lg bg-cyan-400/10 text-cyan-300"><MonitorCog className="size-5" /></div>
            <div><h3 className="text-sm font-medium">Interface density</h3><p className="mt-1 text-xs text-slate-500">Choose spacing for lists and controls.</p></div>
          </div>
          <div className="flex gap-2" aria-label="Interface density">
            <Button size="sm" variant={density === "comfortable" ? "default" : "outline"} onClick={() => setDensity("comfortable")}>Comfortable</Button>
            <Button size="sm" variant={density === "compact" ? "default" : "outline"} onClick={() => setDensity("compact")}>Compact</Button>
          </div>
        </div>
        <div className="mt-4 flex max-w-2xl items-center justify-between gap-6 rounded-xl border border-border bg-surface p-4">
          <div><h3 className="text-sm font-medium">Global bandwidth limit</h3><p className="mt-1 text-xs text-slate-500">MiB/s shared by all active downloads. Use 0 for unlimited.</p></div>
          <div className="flex items-center gap-2">
            <Input className="w-28" aria-label="Global bandwidth limit" type="number" min={0} step="0.25" value={globalLimitMiB} onChange={(event) => setGlobalLimitMiB(event.target.value)} />
            <Button size="sm" variant="outline" onClick={() => void applyGlobalLimit()}>Apply</Button>
          </div>
        </div>
        {limitStatus && <p className="mt-3 text-xs text-slate-400" role="status">{limitStatus}</p>}
        <div className="mt-4 max-w-2xl rounded-xl border border-border bg-surface p-4">
          <div className="flex items-center justify-between"><div><h3 className="text-sm font-medium">Browser integration</h3><p className="mt-1 text-xs text-slate-500">Chrome and Edge use the authenticated FluxDM native host.</p></div><Badge variant="secondary">Extension ID hnem…cbfn</Badge></div>
          <ol className="mt-3 list-decimal space-y-1 pl-5 text-xs text-slate-400"><li>Installed release: open <strong>FluxDM Browser Extension Setup</strong> from the Start menu. Development build: run <code>scripts\install-browser-integration.ps1</code>.</li><li>Enable browser Developer mode and load the packaged <code>browser-extension</code> folder as an unpacked extension.</li><li>Open Extension options and select Test connection; it reports Connected only after the desktop bridge replies.</li></ol>
        </div>
        <SiteProfiles />
		<div className="mt-4 max-w-4xl rounded-xl border border-red-400/15 bg-red-400/[0.03] p-4"><h3 className="text-sm font-medium text-red-100">Privacy reset</h3><p className="mt-1 text-xs text-slate-500">Clears stored credentials, schedule execution history, terminal download history, and local logs. Downloaded files are never deleted.</p><Button className="mt-3" variant="destructive" onClick={()=>{if(window.confirm("Clear FluxDM private data? Downloaded files will remain.")){void clearPrivateData().then(()=>setPrivacyStatus("Private data cleared. Downloaded files were kept.")).catch(()=>setPrivacyStatus("Could not clear private data."))}}}>Clear private data</Button>{privacyStatus&&<p className="mt-2 text-xs text-slate-400" role="status">{privacyStatus}</p>}</div>
      </div>
    </Panel>
  )
}

function Panel({ title, description, badge, children }: { title: string; description: string; badge: string; children: ReactNode }) {
  return (
    <div className="ui-panel rounded-2xl">
      <div className="ui-panel-header flex items-center justify-between px-6 py-4">
        <div><h2 className="font-medium">{title}</h2><p className="mt-1 text-xs text-slate-500">{description}</p></div>
        <Badge variant="secondary">{badge}</Badge>
      </div>
      {children}
    </div>
  )
}
