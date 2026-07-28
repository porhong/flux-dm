import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Activity, ArrowDownToLine, Ban, Download, ExternalLink, FolderOpen, Gauge, Info, LoaderCircle, MoreHorizontal, Pause, Pencil, Play, RefreshCw, RotateCcw, Search, Trash2 } from "lucide-react"
import { EventsOff, EventsOn } from "../../../wailsjs/runtime/runtime"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Progress } from "@/components/ui/progress"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { assignDownloads, cancelDownload, downloadSchema, listCategories, listDownloads, listQueues, moveCompletedDownloadFiles, openCompletedDownloadFile, pauseDownload, progressSchema, recycleCompletedDownloadFiles, removeCompletedDownloadHistory, renameCompletedDownloadFile, revealCompletedDownloadFile, selectDestinationDirectory, restartDownload, resumeDownload, startDownload, type Category, type CompletedFileOperationResult, type DownloadItem, type DownloadQueue, type HealthStatus } from "@/lib/backend"

import { AddDownloadDialog } from "./add-download-dialog"
import { publishProgress, useDownloadProgress } from "./progress-store"

interface DownloadsSectionProps {
  health: HealthStatus | null
  hasBackendError: boolean
  addDialogOpen: boolean
  onAddDialogOpenChange: (open: boolean) => void
}

const rowHeight = 96
const viewportHeight = 560
const overscan = 5
type FileDialog = { kind: "rename" | "move" | "delete" | "openExecutable"; items: DownloadItem[] }

export function DownloadsSection({ health, hasBackendError, addDialogOpen, onAddDialogOpenChange }: DownloadsSectionProps) {
  const [downloads, setDownloads] = useState<DownloadItem[]>([])
  const [loadError, setLoadError] = useState<string | null>(null)
  const [searchText, setSearchText] = useState("")
  const [stateFilter, setStateFilter] = useState("all")
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
  const [properties, setProperties] = useState<DownloadItem | null>(null)
  const [scrollTop, setScrollTop] = useState(0)
  const [categories, setCategories] = useState<Category[]>([])
  const [queues, setQueues] = useState<DownloadQueue[]>([])
  const [bulkCategory, setBulkCategory] = useState("none")
  const [bulkQueue, setBulkQueue] = useState("none")
  const [fileDialog, setFileDialog] = useState<FileDialog | null>(null)
  const [fileMessage, setFileMessage] = useState<string | null>(null)

  useEffect(() => {
    let mounted = true
    void listDownloads().then((items) => { if (mounted) setDownloads(items) }).catch(() => { if (mounted) setLoadError("Could not load download history.") })
    void Promise.all([listCategories(), listQueues()]).then(([nextCategories, nextQueues]) => { if (mounted) { setCategories(nextCategories); setQueues(nextQueues) } }).catch(() => undefined)
    EventsOn("download:updated", (payload: unknown) => {
      const parsed = downloadSchema.safeParse(payload)
      if (parsed.success) upsertDownload(setDownloads, parsed.data)
    })
    EventsOn("download:progress", (payload: unknown) => {
      const parsed = progressSchema.safeParse(payload)
      if (parsed.success) publishProgress(parsed.data)
    })
    return () => {
      mounted = false
      EventsOff("download:updated")
      EventsOff("download:progress")
    }
  }, [])

  const filtered = useMemo(() => {
    const query = searchText.trim().toLocaleLowerCase()
    return downloads.filter((item) => (stateFilter === "all" || item.state === stateFilter) && (!query || item.fileName.toLocaleLowerCase().includes(query) || item.url.toLocaleLowerCase().includes(query)))
  }, [downloads, searchText, stateFilter])
  const active = downloads.filter((item) => ["probing", "preparing", "downloading", "pausing", "retrying"].includes(item.state)).length
  const completed = downloads.filter((item) => item.state === "completed").length
  const toggleSelected = useCallback((id: string) => setSelected((current) => {
    const next = new Set(current)
    if (next.has(id)) next.delete(id); else next.add(id)
    return next
  }), [])
  const selectVisible = () => setSelected((current) => {
    const next = new Set(current)
    const allSelected = filtered.length > 0 && filtered.every((item) => next.has(item.id))
    filtered.forEach((item) => { if (allSelected) next.delete(item.id); else next.add(item.id) })
    return next
  })
  const runBulk = async (action: (item: DownloadItem) => Promise<void>) => {
    const items = downloads.filter((item) => selected.has(item.id))
    await Promise.allSettled(items.map(action))
  }
  const organizeSelected = async () => {
    const ids = [...selected]
    const categoryId = bulkCategory === "none" ? "" : bulkCategory
    const queueId = bulkQueue === "none" ? "" : bulkQueue
    await assignDownloads({ downloadIds: ids, categoryId, queueId, priority: 0 })
    setDownloads((current) => current.map((item) => selected.has(item.id) ? { ...item, categoryId, queueId, priority: 0 } : item))
  }
  const completedSelected = useMemo(() => downloads.filter((item) => selected.has(item.id) && item.state === "completed"), [downloads, selected])
  const applyFileResult = (result: CompletedFileOperationResult) => {
    setDownloads((current) => current.filter((item) => !result.removedIds.includes(item.id)).map((item) => result.updated.find((updated) => updated.id === item.id) ?? item))
    setSelected((current) => new Set([...current].filter((id) => !result.removedIds.includes(id))))
    const details = [result.failures.length ? `${result.failures.length} failed` : "", result.skippedIds.length ? `${result.skippedIds.length} skipped (not completed)` : ""].filter(Boolean)
    setFileMessage(details.length ? `File operation completed: ${details.join(", ")}.` : null)
  }
  const runFileAction = async (action: () => Promise<void>) => {
    setFileMessage(null)
    try { await action() } catch (cause) { setFileMessage(errorMessage(cause)) }
  }
  const requestOpen = (item: DownloadItem) => {
    if (isExecutableName(item.fileName)) { setFileDialog({ kind: "openExecutable", items: [item] }); return }
    void runFileAction(() => openCompletedDownloadFile(item.id))
  }

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "a" && event.target instanceof HTMLElement && !["INPUT", "TEXTAREA"].includes(event.target.tagName)) {
        event.preventDefault(); selectVisible()
      }
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  })

  return (
    <>
      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-3">
        <Metric icon={Activity} label="Active" value={String(active)} detail={active === 1 ? "1 transfer running" : `${active} transfers running`} />
        <Metric icon={Gauge} label="Mode" value="Adaptive" detail="Dynamic bounded segmentation" />
        <Metric icon={Download} label="Completed" value={String(completed)} detail={`${downloads.length} total downloads`} />
      </div>

      <div className="overflow-hidden rounded-2xl border border-white/8 bg-white/[0.025] shadow-2xl shadow-black/20">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/8 px-5 py-4">
          <div><h2 className="font-medium">Transfer queue</h2><p className="mt-1 text-xs text-slate-500">Virtualized history, adaptive segments, and crash-safe recovery.</p></div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative"><Search className="pointer-events-none absolute left-3 top-2.5 size-4 text-slate-500" /><Input className="w-56 pl-9" aria-label="Search downloads" placeholder="Search downloads" value={searchText} onChange={(event) => { setSearchText(event.target.value); setScrollTop(0) }} /></div>
            <Select value={stateFilter} onValueChange={(value) => { setStateFilter(value); setScrollTop(0) }}><SelectTrigger className="w-36" aria-label="Filter status"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">All states</SelectItem>{["downloading", "paused", "completed", "failed", "cancelled"].map((state) => <SelectItem key={state} value={state}>{stateLabel(state as DownloadItem["state"])}</SelectItem>)}</SelectContent></Select>
            <Badge variant={health ? "default" : hasBackendError ? "destructive" : "secondary"}>{health ? "Ready" : hasBackendError ? "Offline" : "Starting"}</Badge>
          </div>
        </div>
        {selected.size > 0 && <div className="flex flex-wrap items-center gap-2 border-b border-white/8 bg-cyan-400/[0.04] px-5 py-2 text-xs"><span className="mr-2 text-cyan-200">{selected.size} selected</span><Button size="sm" variant="ghost" onClick={() => void runBulk((item) => item.state === "downloading" ? pauseDownload(item.id) : Promise.resolve())}><Pause className="size-3.5" /> Pause</Button><Button size="sm" variant="ghost" onClick={() => void runBulk((item) => item.state === "paused" ? resumeDownload(item.id) : Promise.resolve())}><Play className="size-3.5" /> Resume</Button><Button size="sm" variant="ghost" onClick={() => void runBulk((item) => ["queued", "paused", "downloading"].includes(item.state) ? cancelDownload(item.id) : Promise.resolve())}><Ban className="size-3.5" /> Cancel</Button>{completedSelected.length > 0 && <><Button size="sm" variant="ghost" onClick={() => setFileDialog({ kind: "move", items: completedSelected })}><FolderOpen className="size-3.5" /> Move files</Button><Button size="sm" variant="ghost" onClick={() => setFileDialog({ kind: "delete", items: completedSelected })}><Trash2 className="size-3.5" /> Remove files</Button></>}<select aria-label="Bulk category" className="rounded border border-white/10 bg-slate-950 p-1.5" value={bulkCategory} onChange={(event)=>setBulkCategory(event.target.value)}><option value="none">No category</option>{categories.map((item)=><option key={item.id} value={item.id}>{item.name}</option>)}</select><select aria-label="Bulk queue" className="rounded border border-white/10 bg-slate-950 p-1.5" value={bulkQueue} onChange={(event)=>setBulkQueue(event.target.value)}><option value="none">Default queue</option>{queues.map((item)=><option key={item.id} value={item.id}>{item.name}</option>)}</select><Button size="sm" variant="outline" onClick={()=>void organizeSelected()}>Organize</Button><Button size="sm" variant="ghost" onClick={() => setSelected(new Set())}>Clear</Button></div>}
        {loadError && <p className="m-4 rounded-lg border border-red-400/15 bg-red-400/5 p-3 text-sm text-red-200" role="alert">{loadError}</p>}
        {fileMessage && <p className="m-4 rounded-lg border border-amber-400/15 bg-amber-400/5 p-3 text-sm text-amber-100" role="status">{fileMessage}</p>}
        {downloads.length === 0 ? <EmptyDownloads onAdd={() => onAddDialogOpenChange(true)} /> : filtered.length === 0 ? <div className="grid min-h-60 place-items-center text-sm text-slate-500">No downloads match the current filters.</div> : (
          <VirtualDownloadList items={filtered} selected={selected} scrollTop={scrollTop} onScroll={setScrollTop} onToggle={toggleSelected} onProperties={setProperties} onSelectAll={selectVisible} onFileAction={(kind, item) => { if (kind === "open") requestOpen(item); else if (kind === "reveal") void runFileAction(() => revealCompletedDownloadFile(item.id)); else setFileDialog({ kind, items: [item] }) }} />
        )}
      </div>
      <AddDownloadDialog open={addDialogOpen} onOpenChange={onAddDialogOpenChange} onCreated={(item) => upsertDownload(setDownloads, item)} />
      <DownloadProperties item={properties} onOpenChange={(open) => { if (!open) setProperties(null) }} />
      <CompletedFileDialog key={fileDialog ? `${fileDialog.kind}:${fileDialog.items.map((item) => item.id).join(",")}` : "none"} dialog={fileDialog} onOpenChange={(open) => { if (!open) setFileDialog(null) }} onCompleted={(result) => { applyFileResult(result); setFileDialog(null) }} onError={(message) => setFileMessage(message)} />
    </>
  )
}

function VirtualDownloadList({ items, selected, scrollTop, onScroll, onToggle, onProperties, onSelectAll, onFileAction }: { items: DownloadItem[]; selected: Set<string>; scrollTop: number; onScroll: (value: number) => void; onToggle: (id: string) => void; onProperties: (item: DownloadItem) => void; onSelectAll: () => void; onFileAction: (kind: "open" | "reveal" | "rename" | "move" | "delete", item: DownloadItem) => void }) {
  const start = Math.max(0, Math.floor(scrollTop / rowHeight) - overscan)
  const count = Math.ceil(viewportHeight / rowHeight) + overscan * 2
  const visible = items.slice(start, start + count)
  const allSelected = items.length > 0 && items.every((item) => selected.has(item.id))
  return <div role="table" aria-label="Downloads" className="download-list w-full">
    <div role="row" className="download-list-row grid h-11 items-center border-b border-white/8 px-4 text-xs font-medium text-slate-500"><div role="columnheader"><input aria-label="Select all visible downloads" type="checkbox" checked={allSelected} onChange={onSelectAll} /></div><div role="columnheader">File</div><div role="columnheader" className="download-progress-cell">Progress</div><div role="columnheader">Status</div><div role="columnheader" className="text-right">Actions</div></div>
    <div className="relative overflow-auto" style={{ height: Math.min(viewportHeight, items.length * rowHeight) }} onScroll={(event) => onScroll(event.currentTarget.scrollTop)}>
      <div className="relative" style={{ height: items.length * rowHeight }}>
        {visible.map((item, offset) => <div key={item.id} className="absolute inset-x-0" style={{ height: rowHeight, transform: `translateY(${(start + offset) * rowHeight}px)` }}><DownloadRow item={item} selected={selected.has(item.id)} onToggle={onToggle} onProperties={onProperties} onFileAction={onFileAction} /></div>)}
      </div>
    </div>
  </div>
}

const DownloadRow = memo(function DownloadRow({ item, selected, onToggle, onProperties, onFileAction }: { item: DownloadItem; selected: boolean; onToggle: (id: string) => void; onProperties: (item: DownloadItem) => void; onFileAction: (kind: "open" | "reveal" | "rename" | "move" | "delete", item: DownloadItem) => void }) {
  const liveProgress = useDownloadProgress(item.id)
  const [actionError, setActionError] = useState<string | null>(null)
  const downloadedBytes = liveProgress?.downloadedBytes ?? item.downloadedBytes
  const totalBytes = liveProgress?.totalBytes ?? item.totalBytes
  const speed = liveProgress?.speedBytesPerSecond ?? 0
  const eta = liveProgress?.etaSeconds ?? -1
  const percentage = totalBytes > 0 ? Math.min(100, (downloadedBytes / totalBytes) * 100) : 0
  const isCompleted = item.state === "completed"
  const host = useMemo(() => displayHost(item.finalUrl || item.url), [item.finalUrl, item.url])
  const runAction = async (action: () => Promise<void>) => { setActionError(null); try { await action() } catch { setActionError("Action failed") } }
  const primaryAction = item.state === "downloading" ? { label: "Pause", icon: Pause, run: () => pauseDownload(item.id) } : item.state === "paused" && !item.restartRequired ? { label: "Resume", icon: Play, run: () => resumeDownload(item.id) } : item.state === "failed" && !item.restartRequired ? { label: "Retry", icon: RotateCcw, run: () => startDownload(item.id) } : (item.restartRequired || item.state === "cancelled") ? { label: "Restart", icon: RefreshCw, run: () => restartDownload(item.id) } : null
  const PrimaryIcon = primaryAction?.icon
  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === " ") { event.preventDefault(); onToggle(item.id) }
    if (event.key === "Enter") onProperties(item)
    if (event.key.toLowerCase() === "p" && primaryAction) void runAction(primaryAction.run)
    if (event.key === "Delete" && ["queued", "paused", "downloading"].includes(item.state)) void runAction(() => cancelDownload(item.id))
  }
  return <div role="row" tabIndex={0} aria-selected={selected} onKeyDown={onKeyDown} onDoubleClick={() => onProperties(item)} className={`download-list-row ${isCompleted ? "download-list-row--completed" : ""} grid h-full items-center border-b border-white/6 px-4 outline-none focus:bg-cyan-400/[0.05] ${selected ? "bg-cyan-400/[0.04]" : "hover:bg-white/[0.02]"}`}>
    <div role="cell"><input aria-label={`Select ${item.fileName}`} type="checkbox" checked={selected} onChange={() => onToggle(item.id)} /></div>
    <div role="cell" className="min-w-0"><div className="truncate font-medium text-slate-200" title={item.fileName}>{item.fileName}</div><div className="mt-1 truncate text-xs text-slate-500">{host}</div><div className="mt-1 text-[11px] text-slate-600">{item.segmentCount} {item.segmentCount === 1 ? "segment" : "segments"} · {item.connections}×</div></div>
    {!isCompleted && <div role="cell" className="download-progress-cell pr-5"><Progress value={percentage} aria-label={`${item.fileName} progress`} /><div className="mt-1.5 flex justify-between gap-3 text-xs text-slate-500"><span className="truncate">{formatBytes(downloadedBytes)} · {formatRate(speed)}</span><span className="shrink-0">{eta >= 0 ? `${formatDuration(eta)} left` : totalBytes >= 0 ? formatBytes(totalBytes) : "Unknown size"}</span></div></div>}
    <div role="cell"><Badge variant={stateVariant(item.state)}>{stateLabel(item.state)}</Badge>{item.lastError && <div className="mt-1 truncate text-xs text-red-300" title={item.lastError}>{item.lastError}</div>}{actionError && <div className="mt-1 text-xs text-red-300">{actionError}</div>}</div>
    <div role="cell" className="flex justify-end gap-1">{primaryAction && PrimaryIcon && <Button size="sm" variant="outline" aria-label={`${primaryAction.label} ${item.fileName}`} onClick={() => void runAction(primaryAction.run)}><PrimaryIcon className="size-4" /> {primaryAction.label}</Button>}{["probing", "preparing", "pausing", "retrying"].includes(item.state) && <LoaderCircle className="my-auto size-4 animate-spin text-cyan-300" />}<DropdownMenu><DropdownMenuTrigger asChild><Button size="sm" variant="ghost" aria-label={`More actions for ${item.fileName}`}><MoreHorizontal className="size-4" /></Button></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem onSelect={() => onProperties(item)}><Info className="mr-2 size-4" /> Properties</DropdownMenuItem>{item.state === "completed" && <><DropdownMenuSeparator /><DropdownMenuItem onSelect={() => onFileAction("open", item)}><ExternalLink className="mr-2 size-4" /> Open</DropdownMenuItem><DropdownMenuItem onSelect={() => onFileAction("reveal", item)}><FolderOpen className="mr-2 size-4" /> Show in folder</DropdownMenuItem><DropdownMenuItem onSelect={() => onFileAction("rename", item)}><Pencil className="mr-2 size-4" /> Rename</DropdownMenuItem><DropdownMenuItem onSelect={() => onFileAction("move", item)}><FolderOpen className="mr-2 size-4" /> Move</DropdownMenuItem><DropdownMenuItem className="text-red-300" onSelect={() => onFileAction("delete", item)}><Trash2 className="mr-2 size-4" /> Remove</DropdownMenuItem></>}<DropdownMenuSeparator /><DropdownMenuItem disabled={!(["queued", "paused", "downloading"].includes(item.state))} onSelect={() => void runAction(() => cancelDownload(item.id))}><Ban className="mr-2 size-4" /> Cancel</DropdownMenuItem></DropdownMenuContent></DropdownMenu></div>
  </div>
})

function DownloadProperties({ item, onOpenChange }: { item: DownloadItem | null; onOpenChange: (open: boolean) => void }) {
  return <Dialog open={item !== null} onOpenChange={onOpenChange}><DialogContent className="max-w-2xl"><DialogHeader><DialogTitle>Download properties</DialogTitle><DialogDescription>Transfer metadata and local paths.</DialogDescription></DialogHeader>{item && <dl className="grid grid-cols-[140px_1fr] gap-x-4 gap-y-3 text-sm"><Property label="Filename" value={item.fileName} /><Property label="State" value={stateLabel(item.state)} /><Property label="Source" value={item.finalUrl || item.url} /><Property label="Destination" value={item.destinationPath} /><Property label="Temporary file" value={item.tempPath} /><Property label="Size" value={formatBytes(item.totalBytes)} /><Property label="Segments" value={`${item.segmentCount} (${item.connections} configured connections)`} /><Property label="Retries" value={String(item.retryCount)} /><Property label="Created" value={new Date(item.createdAt).toLocaleString()} /></dl>}</DialogContent></Dialog>
}

function CompletedFileDialog({ dialog, onOpenChange, onCompleted, onError }: { dialog: FileDialog | null; onOpenChange: (open: boolean) => void; onCompleted: (result: CompletedFileOperationResult) => void; onError: (message: string) => void }) {
  const [fileName, setFileName] = useState(() => dialog?.kind === "rename" ? dialog.items[0]?.fileName ?? "" : "")
  const [directory, setDirectory] = useState("")
  const [deleteMode, setDeleteMode] = useState<"history" | "recycle">("recycle")
  const [submitting, setSubmitting] = useState(false)
  const close = () => onOpenChange(false)
  const submit = async () => {
    if (!dialog) return
    setSubmitting(true)
    try {
      if (dialog.kind === "rename") {
        const updated = await renameCompletedDownloadFile(dialog.items[0].id, fileName)
        onCompleted({ updated: [updated], removedIds: [], skippedIds: [], failures: [] })
      } else if (dialog.kind === "move") {
        onCompleted(await moveCompletedDownloadFiles({ downloadIds: dialog.items.map((item) => item.id), destinationDir: directory }))
      } else if (dialog.kind === "delete") {
        const ids = dialog.items.map((item) => item.id)
        onCompleted(deleteMode === "recycle" ? await recycleCompletedDownloadFiles(ids) : await removeCompletedDownloadHistory(ids))
      } else {
        await openCompletedDownloadFile(dialog.items[0].id)
        close()
      }
    } catch (cause) { onError(errorMessage(cause)) } finally { setSubmitting(false) }
  }
  const browse = async () => { try { const selected = await selectDestinationDirectory(); if (selected) setDirectory(selected) } catch (cause) { onError(errorMessage(cause)) } }
  if (!dialog) return null
  const countLabel = dialog.items.length === 1 ? dialog.items[0].fileName : `${dialog.items.length} completed files`
  const title = dialog.kind === "rename" ? "Rename completed file" : dialog.kind === "move" ? "Move completed files" : dialog.kind === "delete" ? "Remove completed files" : "Open executable file"
  return <Dialog open onOpenChange={onOpenChange}><DialogContent><DialogHeader><DialogTitle>{title}</DialogTitle><DialogDescription>{dialog.kind === "rename" ? "A conflicting name will receive a unique numeric suffix." : dialog.kind === "move" ? `Choose where to move ${countLabel}. Existing files will not be overwritten.` : dialog.kind === "delete" ? `Choose how to remove ${countLabel}.` : `${countLabel} can run code. FluxDM will only open it because you explicitly confirm this action.`}</DialogDescription></DialogHeader>{dialog.kind === "rename" && <label className="block space-y-2 text-sm"><span>Filename</span><Input aria-label="New filename" value={fileName} onChange={(event) => setFileName(event.target.value)} /></label>}{dialog.kind === "move" && <label className="block space-y-2 text-sm"><span>Destination folder</span><div className="flex gap-2"><Input aria-label="Move destination folder" value={directory} onChange={(event) => setDirectory(event.target.value)} /><Button type="button" variant="outline" onClick={() => void browse()}><FolderOpen className="size-4" /> Browse</Button></div></label>}{dialog.kind === "delete" && <div className="space-y-3 text-sm"><label className="flex items-start gap-2"><input type="radio" checked={deleteMode === "recycle"} onChange={() => setDeleteMode("recycle")} /><span><strong>Move files to Recycle Bin and remove history</strong><br /><span className="text-slate-400">Files can be restored from the Recycle Bin outside FluxDM.</span></span></label><label className="flex items-start gap-2"><input type="radio" checked={deleteMode === "history"} onChange={() => setDeleteMode("history")} /><span><strong>Remove history only</strong><br /><span className="text-slate-400">Keep files in their current locations.</span></span></label></div>}<div className="flex justify-end gap-2"><Button type="button" variant="ghost" onClick={close}>Cancel</Button><Button type="button" variant={dialog.kind === "delete" ? "destructive" : "default"} disabled={submitting || (dialog.kind === "rename" && !fileName.trim()) || (dialog.kind === "move" && !directory.trim())} onClick={() => void submit()}>{submitting && <LoaderCircle className="size-4 animate-spin" />}{dialog.kind === "openExecutable" ? "Open file" : dialog.kind === "delete" ? deleteMode === "recycle" ? "Recycle files" : "Remove history" : dialog.kind === "move" ? "Move files" : "Rename file"}</Button></div></DialogContent></Dialog>
}

function Property({ label, value }: { label: string; value: string }) { return <><dt className="text-slate-500">{label}</dt><dd className="min-w-0 break-all text-slate-200">{value}</dd></> }
function EmptyDownloads({ onAdd }: { onAdd: () => void }) { return <div className="grid min-h-80 place-items-center p-8 text-center"><div className="max-w-sm"><div className="mx-auto mb-5 grid size-16 place-items-center rounded-2xl border border-white/8 bg-white/[0.03] text-slate-500"><ArrowDownToLine className="size-7" /></div><h3 className="text-lg font-medium">No downloads yet</h3><p className="mt-2 text-sm leading-6 text-slate-500">Add an HTTP or HTTPS URL to start a reliable adaptive download.</p><Button className="mt-5" onClick={onAdd}>Add download</Button></div></div> }
function upsertDownload(setDownloads: React.Dispatch<React.SetStateAction<DownloadItem[]>>, item: DownloadItem) { publishProgress({ id: item.id, downloadedBytes: item.downloadedBytes, totalBytes: item.totalBytes, speedBytesPerSecond: 0, etaSeconds: -1 }); setDownloads((current) => [item, ...current.filter((existing) => existing.id !== item.id)].sort((left, right) => right.createdAt.localeCompare(left.createdAt))) }
function Metric({ icon: Icon, label, value, detail }: { icon: typeof Activity; label: string; value: string; detail: string }) { return <div className="rounded-xl border border-white/8 bg-white/[0.025] p-4"><div className="flex items-center justify-between"><span className="text-sm text-slate-400">{label}</span><Icon className="size-4 text-cyan-300" /></div><div className="mt-3 text-2xl font-semibold tracking-tight">{value}</div><div className="mt-1 text-xs text-slate-600">{detail}</div></div> }
function formatBytes(bytes: number): string { if (bytes < 0) return "—"; if (bytes < 1024) return `${bytes} B`; const units = ["KB", "MB", "GB", "TB"]; let value = bytes / 1024; let unit = units[0]; for (let index = 1; value >= 1024 && index < units.length; index++) { value /= 1024; unit = units[index] } return `${value.toFixed(value >= 10 ? 1 : 2)} ${unit}` }
function formatRate(bytes: number): string { return bytes > 0 ? `${formatBytes(bytes)}/s` : "—/s" }
function formatDuration(seconds: number): string { if (seconds < 60) return `${seconds}s`; const minutes = Math.floor(seconds / 60); return `${minutes}m ${seconds % 60}s` }
function displayHost(rawURL: string): string { try { return new URL(rawURL).host } catch { return "Unknown host" } }
function isExecutableName(fileName: string): boolean { return [".exe", ".msi", ".msp", ".msix", ".appx", ".bat", ".cmd", ".com", ".scr", ".ps1", ".vbs", ".js", ".jse", ".wsf", ".hta", ".cpl", ".reg", ".jar"].some((extension) => fileName.toLocaleLowerCase().endsWith(extension)) }
function errorMessage(cause: unknown): string { return cause instanceof Error ? cause.message : typeof cause === "string" ? cause : "FluxDM could not complete the file action." }
function stateLabel(state: DownloadItem["state"]): string { return state.charAt(0).toUpperCase() + state.slice(1) }
function stateVariant(state: DownloadItem["state"]): "default" | "secondary" | "destructive" | "outline" { if (state === "completed") return "default"; if (state === "failed") return "destructive"; if (state === "cancelled") return "outline"; return "secondary" }
