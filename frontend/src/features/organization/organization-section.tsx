import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react"
import { FolderCog, ListOrdered, Plus, Trash2 } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  deleteCategory, deleteQueue, listCategories, listQueues, saveCategory, saveQueue,
  type Category, type DownloadQueue,
} from "@/lib/backend"

const connectionChoices = [1, 2, 4, 8, 16] as const

export function OrganizationSection() {
  const [categories, setCategories] = useState<Category[]>([])
  const [queues, setQueues] = useState<DownloadQueue[]>([])
  const [message, setMessage] = useState("Loading organization rules…")

  const refresh = useCallback(async () => {
    try {
      const [nextCategories, nextQueues] = await Promise.all([listCategories(), listQueues()])
      setCategories(nextCategories); setQueues(nextQueues); setMessage("")
    } catch { setMessage("Could not load organization rules.") }
  }, [])
  useEffect(() => { void Promise.resolve().then(refresh) }, [refresh])

  return (
    <div className="space-y-5">
      {message && <p role="status" className="text-sm text-slate-400">{message}</p>}
      <CategoryPanel categories={categories} onChanged={refresh} />
      <QueuePanel queues={queues} onChanged={refresh} />
    </div>
  )
}

function CategoryPanel({ categories, onChanged }: { categories: Category[]; onChanged: () => Promise<void> }) {
  const [name,setName]=useState(""); const [extensions,setExtensions]=useState(""); const [destination,setDestination]=useState(""); const [priority,setPriority]=useState("0")
  const submit = async (event:FormEvent) => { event.preventDefault(); await saveCategory({id:"",name,extensions:extensions.split(/[,\s]+/),destinationDir:destination,priority:Number(priority)}); setName("");setExtensions("");await onChanged() }
  return <Panel title="File categories" description="Extension rules choose a category and optional destination." icon={FolderCog}>
    <form className="grid gap-3 border-b border-white/8 p-4 md:grid-cols-[1fr_1.3fr_1.5fr_6rem_auto]" onSubmit={(event)=>void submit(event)}>
      <Input aria-label="Category name" placeholder="Archives" value={name} onChange={(e)=>setName(e.target.value)} required />
      <Input aria-label="Extensions" placeholder="zip, 7z, rar" value={extensions} onChange={(e)=>setExtensions(e.target.value)} required />
      <Input aria-label="Category destination" placeholder="Optional destination folder" value={destination} onChange={(e)=>setDestination(e.target.value)} />
      <Input aria-label="Category priority" type="number" min={-1000} max={1000} value={priority} onChange={(e)=>setPriority(e.target.value)} />
      <Button type="submit"><Plus className="size-4" />Add</Button>
    </form>
    <div className="divide-y divide-white/6">
      {categories.length===0 && <Empty text="No category rules. Downloads will use the destination selected when they are added." />}
      {categories.map((category)=><div key={category.id} className="flex items-center justify-between gap-4 p-4">
        <div><p className="font-medium">{category.name} <Badge variant="secondary">priority {category.priority}</Badge></p><p className="mt-1 text-xs text-slate-500">{category.extensions.join(", ")} · {category.destinationDir||"download-selected destination"}</p></div>
        <Button size="icon" variant="ghost" aria-label={`Delete ${category.name}`} onClick={()=>void deleteCategory(category.id).then(onChanged)}><Trash2 className="size-4" /></Button>
      </div>)}
    </div>
  </Panel>
}

function QueuePanel({ queues, onChanged }: { queues: DownloadQueue[]; onChanged: () => Promise<void> }) {
  const [name,setName]=useState("");const [parallel,setParallel]=useState("3");const [connections,setConnections]=useState<typeof connectionChoices[number]>(4);const [priority,setPriority]=useState("0");const [limit,setLimit]=useState("0");const [sequential,setSequential]=useState(false)
  const submit=async(event:FormEvent)=>{event.preventDefault();await saveQueue({id:"",name,priority:Number(priority),maxParallel:Number(parallel),maxConnections:connections,bandwidthLimit:Math.round(Number(limit)*1024*1024),sequential,enabled:true});setName("");await onChanged()}
  return <Panel title="Download queues" description="Queue policy caps parallel work, connections, and bandwidth." icon={ListOrdered}>
    <form className="grid gap-3 border-b border-white/8 p-4 md:grid-cols-[1.2fr_6rem_7rem_6rem_7rem_auto_auto]" onSubmit={(event)=>void submit(event)}>
      <Input aria-label="Queue name" placeholder="Night downloads" value={name} onChange={(e)=>setName(e.target.value)} required />
      <Input aria-label="Max parallel" type="number" min={1} max={16} value={parallel} onChange={(e)=>setParallel(e.target.value)} />
      <select aria-label="Max connections" className="rounded-md border border-white/10 bg-slate-950 px-3 text-sm" value={connections} onChange={(e)=>setConnections(Number(e.target.value) as typeof connections)}>{connectionChoices.map((value)=><option key={value}>{value}</option>)}</select>
      <Input aria-label="Queue priority" type="number" min={-1000} max={1000} value={priority} onChange={(e)=>setPriority(e.target.value)} />
      <Input aria-label="Queue limit MiB per second" type="number" min={0} step="0.25" value={limit} onChange={(e)=>setLimit(e.target.value)} />
      <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={sequential} onChange={(e)=>setSequential(e.target.checked)} />Sequential</label>
      <Button type="submit"><Plus className="size-4" />Add</Button>
    </form>
    <div className="divide-y divide-white/6">
      {queues.length===0&&<Empty text="No custom queues. Unassigned downloads use the three-download default queue." />}
      {queues.map((queue)=><div key={queue.id} className="flex items-center justify-between gap-4 p-4"><div><p className="font-medium">{queue.name} <Badge variant={queue.enabled?"default":"secondary"}>{queue.enabled?"Running":"Stopped"}</Badge></p><p className="mt-1 text-xs text-slate-500">{queue.sequential?"Sequential":`${queue.maxParallel} parallel`} · {queue.maxConnections} connections · {queue.bandwidthLimit?`${(queue.bandwidthLimit/1048576).toFixed(2)} MiB/s`:"unlimited"} · priority {queue.priority}</p></div><Button size="icon" variant="ghost" aria-label={`Delete ${queue.name}`} onClick={()=>void deleteQueue(queue.id).then(onChanged)}><Trash2 className="size-4" /></Button></div>)}
    </div>
  </Panel>
}

function Panel({title,description,icon:Icon,children}:{title:string;description:string;icon:typeof FolderCog;children:ReactNode}) { return <section className="overflow-hidden rounded-2xl border border-white/8 bg-white/[0.025]"><header className="flex items-center gap-3 border-b border-white/8 px-5 py-4"><Icon className="size-5 text-cyan-300"/><div><h2 className="font-medium">{title}</h2><p className="text-xs text-slate-500">{description}</p></div></header>{children}</section> }
function Empty({text}:{text:string}){return <p className="p-6 text-center text-sm text-slate-500">{text}</p>}
