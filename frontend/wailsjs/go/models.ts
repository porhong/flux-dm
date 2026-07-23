export namespace application {
	
	export class AssignDownloadsInput {
	    downloadIds: string[];
	    categoryId: string;
	    queueId: string;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new AssignDownloadsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadIds = source["downloadIds"];
	        this.categoryId = source["categoryId"];
	        this.queueId = source["queueId"];
	        this.priority = source["priority"];
	    }
	}
	export class CompletedFileOperationFailure {
	    id: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CompletedFileOperationFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.message = source["message"];
	    }
	}
	export class DownloadDTO {
	    id: string;
	    url: string;
	    finalUrl: string;
	    fileName: string;
	    destinationPath: string;
	    tempPath: string;
	    state: string;
	    totalBytes: number;
	    downloadedBytes: number;
	    rangeSupported: boolean;
	    restartRequired: boolean;
	    mimeType: string;
	    createdAt: string;
	    startedAt?: string;
	    completedAt?: string;
	    lastError: string;
	    retryCount: number;
	    connections: number;
	    segmentCount: number;
	    bandwidthLimit: number;
	    categoryId: string;
	    queueId: string;
	    queuePosition: number;
	    priority: number;
	    siteProfileId: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.finalUrl = source["finalUrl"];
	        this.fileName = source["fileName"];
	        this.destinationPath = source["destinationPath"];
	        this.tempPath = source["tempPath"];
	        this.state = source["state"];
	        this.totalBytes = source["totalBytes"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.rangeSupported = source["rangeSupported"];
	        this.restartRequired = source["restartRequired"];
	        this.mimeType = source["mimeType"];
	        this.createdAt = source["createdAt"];
	        this.startedAt = source["startedAt"];
	        this.completedAt = source["completedAt"];
	        this.lastError = source["lastError"];
	        this.retryCount = source["retryCount"];
	        this.connections = source["connections"];
	        this.segmentCount = source["segmentCount"];
	        this.bandwidthLimit = source["bandwidthLimit"];
	        this.categoryId = source["categoryId"];
	        this.queueId = source["queueId"];
	        this.queuePosition = source["queuePosition"];
	        this.priority = source["priority"];
	        this.siteProfileId = source["siteProfileId"];
	    }
	}
	export class CompletedFileOperationResult {
	    updated: DownloadDTO[];
	    removedIds: string[];
	    skippedIds: string[];
	    failures: CompletedFileOperationFailure[];
	
	    static createFrom(source: any = {}) {
	        return new CompletedFileOperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = this.convertValues(source["updated"], DownloadDTO);
	        this.removedIds = source["removedIds"];
	        this.skippedIds = source["skippedIds"];
	        this.failures = this.convertValues(source["failures"], CompletedFileOperationFailure);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CreateDownloadInput {
	    url: string;
	    destinationDir: string;
	    fileName: string;
	    connections: number;
	    bandwidthLimit: number;
	    categoryId: string;
	    queueId: string;
	    priority: number;
	    siteProfileId: string;
	    confirmExecutable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateDownloadInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.destinationDir = source["destinationDir"];
	        this.fileName = source["fileName"];
	        this.connections = source["connections"];
	        this.bandwidthLimit = source["bandwidthLimit"];
	        this.categoryId = source["categoryId"];
	        this.queueId = source["queueId"];
	        this.priority = source["priority"];
	        this.siteProfileId = source["siteProfileId"];
	        this.confirmExecutable = source["confirmExecutable"];
	    }
	}
	
	export class DownloadRequestEvent {
	    pendingId: string;
	    url: string;
	    suggestedFilename: string;
	    referrer: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequestEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pendingId = source["pendingId"];
	        this.url = source["url"];
	        this.suggestedFilename = source["suggestedFilename"];
	        this.referrer = source["referrer"];
	    }
	}
	export class HealthStatus {
	    status: string;
	    version: string;
	    platform: string;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.version = source["version"];
	        this.platform = source["platform"];
	        this.checkedAt = source["checkedAt"];
	    }
	}
	export class MoveCompletedDownloadsInput {
	    downloadIds: string[];
	    destinationDir: string;
	
	    static createFrom(source: any = {}) {
	        return new MoveCompletedDownloadsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadIds = source["downloadIds"];
	        this.destinationDir = source["destinationDir"];
	    }
	}
	export class ProbeDTO {
	    url: string;
	    finalUrl: string;
	    fileName: string;
	    totalBytes: number;
	    mimeType: string;
	    etag: string;
	    lastModified: string;
	    rangeSupported: boolean;
	    executableWarning: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProbeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.finalUrl = source["finalUrl"];
	        this.fileName = source["fileName"];
	        this.totalBytes = source["totalBytes"];
	        this.mimeType = source["mimeType"];
	        this.etag = source["etag"];
	        this.lastModified = source["lastModified"];
	        this.rangeSupported = source["rangeSupported"];
	        this.executableWarning = source["executableWarning"];
	    }
	}
	export class SaveCategoryInput {
	    id: string;
	    name: string;
	    extensions: string[];
	    destinationDir: string;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new SaveCategoryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.extensions = source["extensions"];
	        this.destinationDir = source["destinationDir"];
	        this.priority = source["priority"];
	    }
	}
	export class SaveQueueInput {
	    id: string;
	    name: string;
	    priority: number;
	    maxParallel: number;
	    maxConnections: number;
	    bandwidthLimit: number;
	    sequential: boolean;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveQueueInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.priority = source["priority"];
	        this.maxParallel = source["maxParallel"];
	        this.maxConnections = source["maxConnections"];
	        this.bandwidthLimit = source["bandwidthLimit"];
	        this.sequential = source["sequential"];
	        this.enabled = source["enabled"];
	    }
	}
	export class SaveScheduleInput {
	    id: string;
	    name: string;
	    enabled: boolean;
	    weekdays: number[];
	    timeOfDay: string;
	    action: string;
	    queueId: string;
	    speedLimit: number;
	    missedPolicy: string;
	    postAction: string;
	    confirmPowerAction: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveScheduleInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.weekdays = source["weekdays"];
	        this.timeOfDay = source["timeOfDay"];
	        this.action = source["action"];
	        this.queueId = source["queueId"];
	        this.speedLimit = source["speedLimit"];
	        this.missedPolicy = source["missedPolicy"];
	        this.postAction = source["postAction"];
	        this.confirmPowerAction = source["confirmPowerAction"];
	    }
	}
	export class SaveSiteProfileInput {
	    id: string;
	    name: string;
	    hostPattern: string;
	    authType: string;
	    username: string;
	    password: string;
	    bearerToken: string;
	    cookies: string;
	    headers: Record<string, string>;
	    proxyUrl: string;
	    proxyUsername: string;
	    proxyPassword: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveSiteProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.hostPattern = source["hostPattern"];
	        this.authType = source["authType"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.bearerToken = source["bearerToken"];
	        this.cookies = source["cookies"];
	        this.headers = source["headers"];
	        this.proxyUrl = source["proxyUrl"];
	        this.proxyUsername = source["proxyUsername"];
	        this.proxyPassword = source["proxyPassword"];
	    }
	}

}

export namespace organization {
	
	export class Category {
	    id: string;
	    name: string;
	    extensions: string[];
	    destinationDir: string;
	    priority: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.extensions = source["extensions"];
	        this.destinationDir = source["destinationDir"];
	        this.priority = source["priority"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Queue {
	    id: string;
	    name: string;
	    priority: number;
	    maxParallel: number;
	    maxConnections: number;
	    bandwidthLimit: number;
	    sequential: boolean;
	    enabled: boolean;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Queue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.priority = source["priority"];
	        this.maxParallel = source["maxParallel"];
	        this.maxConnections = source["maxConnections"];
	        this.bandwidthLimit = source["bandwidthLimit"];
	        this.sequential = source["sequential"];
	        this.enabled = source["enabled"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace scheduler {
	
	export class History {
	    id: number;
	    scheduleId: string;
	    runKey: string;
	    // Go type: time
	    scheduledFor: any;
	    // Go type: time
	    executedAt: any;
	    status: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new History(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.scheduleId = source["scheduleId"];
	        this.runKey = source["runKey"];
	        this.scheduledFor = this.convertValues(source["scheduledFor"], null);
	        this.executedAt = this.convertValues(source["executedAt"], null);
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Schedule {
	    id: string;
	    name: string;
	    enabled: boolean;
	    weekdays: number[];
	    timeOfDay: string;
	    action: string;
	    queueId: string;
	    speedLimit: number;
	    missedPolicy: string;
	    postAction: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Schedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.weekdays = source["weekdays"];
	        this.timeOfDay = source["timeOfDay"];
	        this.action = source["action"];
	        this.queueId = source["queueId"];
	        this.speedLimit = source["speedLimit"];
	        this.missedPolicy = source["missedPolicy"];
	        this.postAction = source["postAction"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace siteprofile {
	
	export class Profile {
	    id: string;
	    name: string;
	    hostPattern: string;
	    authType: string;
	    proxyUrl: string;
	    hasCredentials: boolean;
	    hasCookies: boolean;
	    headerNames: string[];
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.hostPattern = source["hostPattern"];
	        this.authType = source["authType"];
	        this.proxyUrl = source["proxyUrl"];
	        this.hasCredentials = source["hasCredentials"];
	        this.hasCookies = source["hasCookies"];
	        this.headerNames = source["headerNames"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

