import { useCallback, useEffect, useState } from "react"
import { CheckCircle2, Download, LoaderCircle, RefreshCw, ShieldAlert } from "lucide-react"
import { Events } from "@wailsio/runtime"

import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { checkForUpdates, downloadUpdate, getUpdateStatus, installPreparedUpdate, saveUpdatePreferences, type UpdateStatus } from "@/lib/backend"

type UpdateAction = "checking" | "downloading" | "installing" | "saving" | null

function isUpdateStatus(value: unknown): value is UpdateStatus {
  return typeof value === "object" && value !== null && "phase" in value && "channel" in value
}

export function UpdateSettings() {
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  const [message, setMessage] = useState("")
  const [activeAction, setActiveAction] = useState<UpdateAction>(null)
  const refresh = useCallback(async () => {
    try { setStatus(await getUpdateStatus()); setMessage("") }
    catch (cause) { setMessage(updateErrorMessage(cause)) }
  }, [])

  useEffect(() => {
    void Promise.resolve().then(refresh)
    Events.On("update:changed", (value: unknown) => {
      const status = eventData(value)
      if (isUpdateStatus(status)) setStatus(status)
    })
    return () => Events.Off("update:changed")
  }, [refresh])

  const perform = async (action: Exclude<UpdateAction, null>, operation: () => Promise<UpdateStatus>) => {
    setActiveAction(action)
    setMessage("")
    try {
      setStatus(await operation())
    } catch (cause) {
      setMessage(updateErrorMessage(cause))
    } finally {
      setActiveAction(null)
    }
  }

  const save = async (channel: "stable" | "preview", autoDownload: boolean) => perform("saving", () => saveUpdatePreferences({ channel, autoDownload }))
  const install = async () => {
    if (!status) return
    if (status.preview && !window.confirm("This is an unsigned preview installer. Continue only if you are testing FluxDM.")) return
    setActiveAction("installing")
    setMessage("")
    try {
      await installPreparedUpdate(status.preview)
    } catch {
      setMessage("Could not start the verified update installer.")
      setActiveAction(null)
    }
  }

  const operationInProgress = activeAction !== null || status?.phase === "checking" || status?.phase === "downloading" || status?.phase === "installing"

  return <section className="mt-4 max-w-4xl overflow-hidden rounded-xl border border-white/8 bg-slate-950/40" aria-label="Application updates">
    <header className="flex items-center gap-3 border-b border-white/8 p-4"><RefreshCw className="size-5 text-cyan-300" /><div><h3 className="text-sm font-medium">Application updates</h3><p className="text-xs text-slate-500">Verified updates install silently, then FluxDM restarts automatically.</p></div></header>
    {!status ? <p className="p-4 text-xs text-slate-500">{message || "Checking update configuration…"}</p> : <div className="space-y-4 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3"><div><p className="text-sm font-medium">FluxDM {status.currentVersion}</p><p className="mt-1 text-xs text-slate-500">{status.lastCheckedAt ? `Last checked ${new Date(status.lastCheckedAt).toLocaleString()}` : "Not checked yet"}</p></div><Button size="sm" variant="outline" disabled={operationInProgress} aria-busy={activeAction === "checking" || status.phase === "checking"} onClick={() => void perform("checking", checkForUpdates)}>{activeAction === "checking" || status.phase === "checking" ? <><LoaderCircle className="size-4 animate-spin" />Checking…</> : <><RefreshCw className="size-4" />Check now</>}</Button></div>
      {status.installedVersion && <p className="rounded-lg border border-emerald-300/20 bg-emerald-300/5 p-3 text-xs text-emerald-200" role="status">Updated to FluxDM {status.installedVersion}{status.installedAt ? ` on ${new Date(status.installedAt).toLocaleString()}` : ""}.</p>}
      <label className="flex items-center justify-between gap-4 rounded-xl border border-border bg-surface p-4"><span><span className="block text-sm font-medium">Preview updates</span><span className="mt-1 block text-xs text-slate-500">Receive release candidates for testing. Preview installers are unsigned and never update automatically.</span></span><input aria-label="Preview updates" type="checkbox" checked={status.channel === "preview"} disabled={operationInProgress} onChange={(event) => void save(event.target.checked ? "preview" : "stable", status.autoDownload)} /></label>
      <label className="flex items-center justify-between gap-4 rounded-xl border border-border bg-surface p-4"><span><span className="block text-sm font-medium">Download stable updates automatically</span><span className="mt-1 block text-xs text-slate-500">Checks daily while FluxDM is running, including when it is hidden in the tray.</span></span><input aria-label="Download stable updates automatically" type="checkbox" checked={status.autoDownload} disabled={operationInProgress} onChange={(event) => void save(status.channel, event.target.checked)} /></label>
      {status.availableVersion && <div className="rounded-xl border border-cyan-300/20 bg-cyan-300/5 p-4"><div className="flex flex-wrap items-center justify-between gap-3"><div><p className="text-sm font-medium">FluxDM {status.availableVersion} is available</p><p className="mt-1 text-xs text-slate-400">{status.preview ? "Preview release — installer confirmation is required." : "Verified production update."}</p></div>{status.releaseNotesUrl && <a className="text-xs text-cyan-300 underline" href={status.releaseNotesUrl}>Release notes</a>}</div>{status.phase === "available" && <Button className="mt-3" size="sm" disabled={operationInProgress} aria-busy={activeAction === "downloading"} onClick={() => void perform("downloading", downloadUpdate)}>{activeAction === "downloading" ? <><LoaderCircle className="size-4 animate-spin" />Starting download…</> : <><Download className="size-4" />Download update</>}</Button>}{status.canInstall && <Button className="mt-3" size="sm" disabled={operationInProgress} aria-busy={activeAction === "installing" || status.phase === "installing"} onClick={() => void install()}>{activeAction === "installing" || status.phase === "installing" ? <><LoaderCircle className="size-4 animate-spin" />Preparing restart…</> : <>{status.preview ? <ShieldAlert className="size-4" /> : <CheckCircle2 className="size-4" />}{status.lastError ? "Retry restart and install" : status.preview ? "Install preview" : "Restart and install"}</>}</Button>}</div>}
      {status.phase === "downloading" && <UpdateDownloadProgress status={status} />}
      {(message || status.lastError) && <p className="text-xs text-red-300" role="alert">{message || status.lastError}</p>}
    </div>}
  </section>
}

function eventData(payload: unknown): unknown {
  return typeof payload === "object" && payload !== null && "data" in payload ? payload.data : payload
}

function updateErrorMessage(cause: unknown): string {
  const message = cause instanceof Error ? cause.message : typeof cause === "string" ? cause : typeof cause === "object" && cause !== null && "message" in cause && typeof cause.message === "string" ? cause.message : ""
  if (message) return message.replace(/^(?:invalid_input|unavailable|internal):\s*/, "")
  return "The update operation could not be completed. Check your connection and try again."
}

function UpdateDownloadProgress({ status }: { status: UpdateStatus }) {
  const percentage = status.totalBytes > 0 ? Math.min(100, Math.round((status.downloadedBytes / status.totalBytes) * 100)) : 0

  return <div className="space-y-2 rounded-xl border border-cyan-300/20 bg-cyan-300/5 p-4" role="status" aria-live="polite">
    <div className="flex items-center justify-between gap-3 text-xs text-cyan-100"><span className="flex items-center gap-2"><LoaderCircle className="size-4 animate-spin" />Downloading and verifying update</span><span className="shrink-0 tabular-nums">{status.totalBytes > 0 ? `${percentage}%` : "Working…"}</span></div>
    <Progress value={percentage} aria-label="Update download progress" />
    <p className="text-xs text-slate-400">{status.totalBytes > 0 ? `${formatBytes(status.downloadedBytes)} of ${formatBytes(status.totalBytes)}` : "Calculating download size…"}</p>
  </div>
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  const units = ["KB", "MB", "GB"]
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length)
  return `${(bytes / 1024 ** exponent).toFixed(exponent === 1 ? 0 : 1)} ${units[exponent - 1]}`
}
