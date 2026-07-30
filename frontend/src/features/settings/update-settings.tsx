import { useCallback, useEffect, useState } from "react"
import { CheckCircle2, Download, RefreshCw, ShieldAlert } from "lucide-react"
import { Events } from "@wailsio/runtime"

import { Button } from "@/components/ui/button"
import { checkForUpdates, downloadUpdate, getUpdateStatus, installPreparedUpdate, saveUpdatePreferences, type UpdateStatus } from "@/lib/backend"

function isUpdateStatus(value: unknown): value is UpdateStatus {
  return typeof value === "object" && value !== null && "phase" in value && "channel" in value
}

export function UpdateSettings() {
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  const [message, setMessage] = useState("")
  const [busy, setBusy] = useState(false)
  const refresh = useCallback(async () => {
    try { setStatus(await getUpdateStatus()); setMessage("") }
    catch { setMessage("Updates are unavailable in this development or unsigned build.") }
  }, [])
  useEffect(() => {
    void Promise.resolve().then(refresh)
    Events.On("update:changed", (value: unknown) => { if (isUpdateStatus(value)) setStatus(value) })
    return () => Events.Off("update:changed")
  }, [refresh])
  const perform = async (action: () => Promise<UpdateStatus>) => {
    setBusy(true)
    try { setStatus(await action()); setMessage("") } catch { setMessage("The update operation could not be completed. Check your connection and try again.") } finally { setBusy(false) }
  }
  const save = async (channel: "stable" | "preview", autoDownload: boolean) => perform(() => saveUpdatePreferences({ channel, autoDownload }))
  const install = async () => {
    if (!status) return
    if (status.preview && !window.confirm("This is an unsigned preview installer. Continue only if you are testing FluxDM.")) return
    setBusy(true)
    try { await installPreparedUpdate(status.preview) } catch { setMessage("Could not start the verified update installer."); setBusy(false) }
  }
  return <section className="mt-4 max-w-4xl overflow-hidden rounded-xl border border-white/8 bg-slate-950/40" aria-label="Application updates">
    <header className="flex items-center gap-3 border-b border-white/8 p-4"><RefreshCw className="size-5 text-cyan-300" /><div><h3 className="text-sm font-medium">Application updates</h3><p className="text-xs text-slate-500">Verified updates install silently, then FluxDM restarts automatically.</p></div></header>
    {!status ? <p className="p-4 text-xs text-slate-500">{message || "Checking update configuration…"}</p> : <div className="space-y-4 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3"><div><p className="text-sm font-medium">FluxDM {status.currentVersion}</p><p className="mt-1 text-xs text-slate-500">{status.lastCheckedAt ? `Last checked ${new Date(status.lastCheckedAt).toLocaleString()}` : "Not checked yet"}</p></div><Button size="sm" variant="outline" disabled={busy || status.phase === "checking"} onClick={() => void perform(checkForUpdates)}><RefreshCw className="size-4" />Check now</Button></div>
      {status.installedVersion && <p className="rounded-lg border border-emerald-300/20 bg-emerald-300/5 p-3 text-xs text-emerald-200" role="status">Updated to FluxDM {status.installedVersion}{status.installedAt ? ` on ${new Date(status.installedAt).toLocaleString()}` : ""}.</p>}
      <label className="flex items-center justify-between gap-4 rounded-xl border border-border bg-surface p-4"><span><span className="block text-sm font-medium">Preview updates</span><span className="mt-1 block text-xs text-slate-500">Receive release candidates for testing. Preview installers are unsigned and never update automatically.</span></span><input aria-label="Preview updates" type="checkbox" checked={status.channel === "preview"} disabled={busy} onChange={(event) => void save(event.target.checked ? "preview" : "stable", status.autoDownload)} /></label>
      <label className="flex items-center justify-between gap-4 rounded-xl border border-border bg-surface p-4"><span><span className="block text-sm font-medium">Download stable updates automatically</span><span className="mt-1 block text-xs text-slate-500">Checks daily while FluxDM is running, including when it is hidden in the tray.</span></span><input aria-label="Download stable updates automatically" type="checkbox" checked={status.autoDownload} disabled={busy} onChange={(event) => void save(status.channel, event.target.checked)} /></label>
      {status.availableVersion && <div className="rounded-xl border border-cyan-300/20 bg-cyan-300/5 p-4"><div className="flex flex-wrap items-center justify-between gap-3"><div><p className="text-sm font-medium">FluxDM {status.availableVersion} is available</p><p className="mt-1 text-xs text-slate-400">{status.preview ? "Preview release — installer confirmation is required." : "Verified production update."}</p></div>{status.releaseNotesUrl && <a className="text-xs text-cyan-300 underline" href={status.releaseNotesUrl}>Release notes</a>}</div>{status.phase === "available" && <Button className="mt-3" size="sm" disabled={busy} onClick={() => void perform(downloadUpdate)}><Download className="size-4" />Download update</Button>}{status.canInstall && <Button className="mt-3" size="sm" disabled={busy} onClick={() => void install()}>{status.preview ? <ShieldAlert className="size-4" /> : <CheckCircle2 className="size-4" />}{status.lastError ? "Retry restart and install" : status.preview ? "Install preview" : "Restart and install"}</Button>}</div>}
      {status.phase === "downloading" && <p className="text-xs text-cyan-200" role="status">Downloading verified update…</p>}
      {(message || status.lastError) && <p className="text-xs text-red-300" role="alert">{message || status.lastError}</p>}
    </div>}
  </section>
}
