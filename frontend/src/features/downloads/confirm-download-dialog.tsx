import { useEffect, useState } from "react"
import { FileDown, FolderOpen, LoaderCircle } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  confirmBrowserDownload,
  defaultDownloadDirectory,
  discardBrowserDownload,
  probeURL,
  selectDestinationDirectory,
  startDownload,
  type DownloadRequestEvent,
  type ProbeResult,
} from "@/lib/backend"

interface ConfirmDownloadDialogProps {
  request: DownloadRequestEvent | null
  onClose: () => void
}

// This component is rendered in its own Wails window, so it deliberately
// presents a window surface rather than nesting a web dialog inside it.
export function ConfirmDownloadDialog({ request, onClose }: ConfirmDownloadDialogProps) {
  const [destinationDir, setDestinationDir] = useState("")
  const [fileName, setFileName] = useState(() => request?.suggestedFilename ?? "")
  const [probe, setProbe] = useState<ProbeResult | null>(null)
  const [probing, setProbing] = useState(() => request !== null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // A resolved request has either been discarded or converted to a download.
  // Keeping this local guard prevents duplicate confirmation calls.
  const [resolved, setResolved] = useState(false)

  useEffect(() => {
    if (!request) return
    let active = true
    void defaultDownloadDirectory()
      .then((dir) => { if (active) setDestinationDir(dir) })
      .catch((cause) => { if (active) setError(errorMessage(cause)) })
    void probeURL(request.url)
      .then((result) => {
        if (!active) return
        setProbe(result)
        if (!request.suggestedFilename) setFileName(result.fileName)
      })
      .catch((cause) => { if (active) setError(errorMessage(cause)) })
      .finally(() => { if (active) setProbing(false) })
    return () => { active = false }
  }, [request])

  const chooseDirectory = async () => {
    setError(null)
    try {
      const dir = await selectDestinationDirectory()
      if (dir) setDestinationDir(dir)
    } catch (cause) {
      setError(errorMessage(cause))
    }
  }

  const start = async () => {
    if (!request || resolved || !probe) return
    if (!destinationDir.trim()) {
      setError("Choose a destination folder.")
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      const created = await confirmBrowserDownload(
        request.pendingId,
        destinationDir.trim(),
        fileName.trim() || request.suggestedFilename,
        4,
      )
      setResolved(true)
      try {
        await startDownload(created.id)
        onClose()
      } catch {
        setError("The download was added but could not start. You can retry it from the downloads list.")
      }
    } catch (cause) {
      setError(errorMessage(cause))
    } finally {
      setSubmitting(false)
    }
  }

  const cancel = async () => {
    if (request && !resolved) {
      setResolved(true)
      await discardBrowserDownload(request.pendingId)
    }
    onClose()
  }

  const source = sourceHost(probe?.finalUrl || request?.url || "")
  const canStart = !submitting && !probing && probe !== null && destinationDir.trim() !== "" && !resolved

  return (
    <main className="browser-confirmation-window flex h-screen min-h-0 flex-col overflow-hidden bg-canvas px-6 pb-5 pt-6" aria-labelledby="browser-download-title">
      <header className="border-l-2 border-primary pl-3">
        <h1 className="text-lg font-semibold tracking-tight text-foreground" id="browser-download-title">Ready to download</h1>
        <p className="mt-1 text-sm leading-5 text-slate-400">Choose a folder and add this file to your transfer queue.</p>
      </header>

      <div className="min-h-0 space-y-4 pt-5">
        <section className="flex min-w-0 items-center gap-3 rounded-xl border border-border bg-surface p-3" aria-label="Browser download details">
          <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-cyan-200"><FileDown className="size-4" /></div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-slate-100" title={fileName || "Download"}>{fileName || "Download"}</p>
            <p className="mt-0.5 truncate text-xs text-slate-400" title={source}>{source || "Unknown source"}</p>
          </div>
          <div className="shrink-0 border-l border-border pl-3 text-right text-xs text-slate-400">
            <p>{probing ? "Inspecting…" : probe ? formatBytes(probe.totalBytes) : "Unavailable"}</p>
            {!probing && probe?.mimeType ? <p className="mt-0.5 max-w-24 truncate text-slate-500" title={probe.mimeType}>{probe.mimeType}</p> : null}
          </div>
        </section>

        <div className="space-y-2">
          <div className="flex min-w-0 items-center justify-between gap-3">
            <label className="text-sm font-medium text-slate-200" htmlFor="browser-download-destination">Save to</label>
            <span className="text-xs text-slate-500">Downloads by default</span>
          </div>
          <div className="flex min-w-0 gap-2">
            <Input className="min-w-0 flex-1" id="browser-download-destination" aria-label="Destination folder" value={destinationDir} onChange={(event) => setDestinationDir(event.target.value)} autoFocus />
            <Button className="shrink-0" type="button" variant="outline" aria-label="Browse destination folder" onClick={() => void chooseDirectory()} disabled={submitting}><FolderOpen className="size-4" /><span className="sr-only">Browse destination folder</span></Button>
          </div>
        </div>

        {error ? <p className="rounded-lg border border-red-400/20 bg-red-400/5 p-3 text-sm text-red-200" role="alert">{error}</p> : null}
      </div>

      <footer className="mt-auto flex items-center gap-3 border-t border-border pt-4">
        <Button className="mr-auto" type="button" variant="ghost" onClick={() => void cancel()} disabled={submitting}>{resolved ? "Close" : "Cancel"}</Button>
        <Button type="button" onClick={() => void start()} disabled={!canStart}>
          {(submitting || probing) && <LoaderCircle className="size-4 animate-spin" />} Start download
        </Button>
      </footer>
    </main>
  )
}

function errorMessage(cause: unknown): string {
  if (cause instanceof Error) return cause.message
  if (typeof cause === "string") return cause
  return "FluxDM could not complete that action."
}

function sourceHost(rawURL: string): string {
  try {
    return new URL(rawURL).host
  } catch {
    return ""
  }
}

function formatBytes(bytes: number): string {
  if (bytes < 0) return "Unknown size"
  if (bytes < 1024) return `${bytes} B`
  const units = ["KB", "MB", "GB", "TB"]
  let value = bytes / 1024
  let unit = units[0]
  for (let index = 1; value >= 1024 && index < units.length; index++) {
    value /= 1024
    unit = units[index]
  }
  return `${value.toFixed(value >= 10 ? 1 : 2)} ${unit}`
}
