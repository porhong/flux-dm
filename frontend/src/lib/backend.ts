import { z } from "zod"
import {
  CancelDownload as invokeCancelDownload,
  AssignDownloads as invokeAssignDownloads,
  ClearSiteProfileSecrets as invokeClearSiteProfileSecrets,
  ClearPrivateData as invokeClearPrivateData,
  CreateDownload as invokeCreateDownload,
  DefaultDownloadDirectory as invokeDefaultDownloadDirectory,
  DeleteCategory as invokeDeleteCategory,
  DeleteQueue as invokeDeleteQueue,
  DeleteSchedule as invokeDeleteSchedule,
  DeleteSiteProfile as invokeDeleteSiteProfile,
  HealthCheck as invokeHealthCheck,
  ListDownloads as invokeListDownloads,
  ListCategories as invokeListCategories,
  ListQueues as invokeListQueues,
  ListScheduleHistory as invokeListScheduleHistory,
  ListSchedules as invokeListSchedules,
  ListSiteProfiles as invokeListSiteProfiles,
  PauseDownload as invokePauseDownload,
  ProbeURL as invokeProbeURL,
  RestartDownload as invokeRestartDownload,
  ResumeDownload as invokeResumeDownload,
  SaveCategory as invokeSaveCategory,
  SaveQueue as invokeSaveQueue,
  SaveSchedule as invokeSaveSchedule,
  SaveSiteProfile as invokeSaveSiteProfile,
  SetDownloadBandwidthLimit as invokeSetDownloadBandwidthLimit,
  SetGlobalBandwidthLimit as invokeSetGlobalBandwidthLimit,
  SelectDestinationDirectory as invokeSelectDestinationDirectory,
  StartDownload as invokeStartDownload,
} from "../../wailsjs/go/main/App"

const healthStatusSchema = z.object({
  status: z.literal("ok"),
  version: z.string(),
  platform: z.string(),
  checkedAt: z.string(),
})

export const downloadStateSchema = z.enum([
  "queued",
  "probing",
  "preparing",
  "downloading",
  "pausing",
  "paused",
  "retrying",
  "completed",
  "failed",
  "cancelled",
])

export const downloadSchema = z.object({
  id: z.string(),
  url: z.string(),
  finalUrl: z.string(),
  fileName: z.string(),
  destinationPath: z.string(),
  tempPath: z.string(),
  state: downloadStateSchema,
  totalBytes: z.number(),
  downloadedBytes: z.number(),
  rangeSupported: z.boolean(),
  restartRequired: z.boolean(),
  mimeType: z.string(),
  createdAt: z.string(),
  // Wails serializes nil Go *string fields as JSON null. Downloads have no
  // start or completion time while they are queued, so these fields must
  // accept null; accepting omission also preserves compatibility with older
  // bindings that left nil pointer fields out of the payload.
  startedAt: z.string().nullish(),
  completedAt: z.string().nullish(),
  lastError: z.string(),
  retryCount: z.number(),
  connections: z.number(),
  segmentCount: z.number(),
  bandwidthLimit: z.number(),
  categoryId: z.string(),
  queueId: z.string(),
  queuePosition: z.number(),
  priority: z.number(),
  siteProfileId: z.string(),
})

export const progressSchema = z.object({
  id: z.string(),
  downloadedBytes: z.number(),
  totalBytes: z.number(),
  speedBytesPerSecond: z.number(),
  etaSeconds: z.number(),
})

const probeSchema = z.object({
  url: z.string(),
  finalUrl: z.string(),
  fileName: z.string(),
  totalBytes: z.number(),
  mimeType: z.string(),
  etag: z.string(),
  lastModified: z.string(),
  rangeSupported: z.boolean(),
	executableWarning:z.boolean(),
})

export type HealthStatus = z.infer<typeof healthStatusSchema>
export type DownloadItem = z.infer<typeof downloadSchema>
export type DownloadProgress = z.infer<typeof progressSchema>
export type ProbeResult = z.infer<typeof probeSchema>

const categorySchema = z.object({
  id: z.string(), name: z.string(), extensions: z.array(z.string()),
  destinationDir: z.string(), priority: z.number(), createdAt: z.string(),
})
const queueSchema = z.object({
  id: z.string(), name: z.string(), priority: z.number(), maxParallel: z.number(),
  maxConnections: z.number(), bandwidthLimit: z.number(), sequential: z.boolean(),
  enabled: z.boolean(), createdAt: z.string(),
})
export type Category = z.infer<typeof categorySchema>
export type DownloadQueue = z.infer<typeof queueSchema>
const scheduleActionSchema = z.enum(["start_queue", "stop_queue", "speed_profile", "retry_failed"])
const missedPolicySchema = z.enum(["skip", "run_once"])
const postActionSchema = z.enum(["none", "exit", "sleep", "hibernate", "shutdown"])
const scheduleSchema = z.object({ id:z.string(),name:z.string(),enabled:z.boolean(),weekdays:z.array(z.number()),timeOfDay:z.string(),action:scheduleActionSchema,queueId:z.string(),speedLimit:z.number(),missedPolicy:missedPolicySchema,postAction:postActionSchema,createdAt:z.string() })
const scheduleHistorySchema = z.object({id:z.number(),scheduleId:z.string(),runKey:z.string(),scheduledFor:z.string(),executedAt:z.string(),status:z.string(),detail:z.string()})
export type Schedule = z.infer<typeof scheduleSchema>
export type ScheduleHistory = z.infer<typeof scheduleHistorySchema>
const siteProfileSchema=z.object({id:z.string(),name:z.string(),hostPattern:z.string(),authType:z.enum(["none","basic","bearer"]),proxyUrl:z.string(),hasCredentials:z.boolean(),hasCookies:z.boolean(),headerNames:z.array(z.string()),createdAt:z.string()})
export type SiteProfile=z.infer<typeof siteProfileSchema>

export interface CreateDownloadInput {
  url: string
  destinationDir: string
  fileName: string
  connections: 1 | 2 | 4 | 8 | 16
  bandwidthLimit: number
	categoryId?: string
	queueId?: string
	priority?: number
	siteProfileId?: string
	confirmExecutable?:boolean
}

export interface SaveCategoryInput { id: string; name: string; extensions: string[]; destinationDir: string; priority: number }
export interface SaveQueueInput { id: string; name: string; priority: number; maxParallel: number; maxConnections: 1 | 2 | 4 | 8 | 16; bandwidthLimit: number; sequential: boolean; enabled: boolean }
export interface AssignDownloadsInput { downloadIds: string[]; categoryId: string; queueId: string; priority: number }
export interface SaveScheduleInput { id:string;name:string;enabled:boolean;weekdays:number[];timeOfDay:string;action:z.infer<typeof scheduleActionSchema>;queueId:string;speedLimit:number;missedPolicy:z.infer<typeof missedPolicySchema>;postAction:z.infer<typeof postActionSchema>;confirmPowerAction:boolean }
export interface SaveSiteProfileInput {id:string;name:string;hostPattern:string;authType:"none"|"basic"|"bearer";username:string;password:string;bearerToken:string;cookies:string;headers:Record<string,string>;proxyUrl:string;proxyUsername:string;proxyPassword:string}

export async function healthCheck(): Promise<HealthStatus> {
  return healthStatusSchema.parse(await invokeHealthCheck())
}

export async function probeURL(url: string): Promise<ProbeResult> {
  return probeSchema.parse(await invokeProbeURL(url))
}

export async function createDownload(input: CreateDownloadInput): Promise<DownloadItem> {
  return downloadSchema.parse(await invokeCreateDownload({ ...input, categoryId: input.categoryId ?? "", queueId: input.queueId ?? "", priority: input.priority ?? 0, siteProfileId:input.siteProfileId??"",confirmExecutable:input.confirmExecutable??false }))
}

export async function defaultDownloadDirectory(): Promise<string> {
  return z.string().min(1).parse(await invokeDefaultDownloadDirectory())
}

export async function listCategories(): Promise<Category[]> { return z.array(categorySchema).parse(await invokeListCategories()) }
export async function saveCategory(input: SaveCategoryInput): Promise<Category> { return categorySchema.parse(await invokeSaveCategory(input)) }
export async function deleteCategory(id: string): Promise<void> { await invokeDeleteCategory(id) }
export async function listQueues(): Promise<DownloadQueue[]> { return z.array(queueSchema).parse(await invokeListQueues()) }
export async function saveQueue(input: SaveQueueInput): Promise<DownloadQueue> { return queueSchema.parse(await invokeSaveQueue(input)) }
export async function deleteQueue(id: string): Promise<void> { await invokeDeleteQueue(id) }
export async function assignDownloads(input: AssignDownloadsInput): Promise<void> { await invokeAssignDownloads(input) }
export async function listSchedules():Promise<Schedule[]>{return z.array(scheduleSchema).parse(await invokeListSchedules())}
export async function saveSchedule(input:SaveScheduleInput):Promise<Schedule>{return scheduleSchema.parse(await invokeSaveSchedule(input))}
export async function deleteSchedule(id:string):Promise<void>{await invokeDeleteSchedule(id)}
export async function listScheduleHistory(limit=100):Promise<ScheduleHistory[]>{return z.array(scheduleHistorySchema).parse(await invokeListScheduleHistory(limit))}
export async function listSiteProfiles():Promise<SiteProfile[]>{return z.array(siteProfileSchema).parse(await invokeListSiteProfiles())}
export async function saveSiteProfile(input:SaveSiteProfileInput):Promise<SiteProfile>{return siteProfileSchema.parse(await invokeSaveSiteProfile(input))}
export async function deleteSiteProfile(id:string):Promise<void>{await invokeDeleteSiteProfile(id)}
export async function clearSiteProfileSecrets(id:string):Promise<void>{await invokeClearSiteProfileSecrets(id)}
export async function clearPrivateData():Promise<void>{await invokeClearPrivateData()}

export async function listDownloads(): Promise<DownloadItem[]> {
  return z.array(downloadSchema).parse(await invokeListDownloads())
}

export async function startDownload(id: string): Promise<void> {
  await invokeStartDownload(id)
}

export async function cancelDownload(id: string): Promise<void> {
  await invokeCancelDownload(id)
}

export async function pauseDownload(id: string): Promise<void> {
  await invokePauseDownload(id)
}

export async function resumeDownload(id: string): Promise<void> {
  await invokeResumeDownload(id)
}

export async function restartDownload(id: string): Promise<void> {
  await invokeRestartDownload(id)
}

export async function setGlobalBandwidthLimit(limit: number): Promise<void> {
  await invokeSetGlobalBandwidthLimit(limit)
}

export async function setDownloadBandwidthLimit(id: string, limit: number): Promise<void> {
  await invokeSetDownloadBandwidthLimit(id, limit)
}

export async function selectDestinationDirectory(): Promise<string> {
  return z.string().parse(await invokeSelectDestinationDirectory())
}
