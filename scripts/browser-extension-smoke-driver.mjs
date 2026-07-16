import { createHash } from "node:crypto"
import { createServer } from "node:http"
import { mkdir, readFile, readdir } from "node:fs/promises"
import { isAbsolute, join, relative, resolve } from "node:path"
import process from "node:process"

const [profileDirectory, extensionID, downloadDirectory, mode = "connected", timeoutArgument = "30000"] = process.argv.slice(2)
if (!profileDirectory || !/^[a-p]{32}$/.test(extensionID || "") || !downloadDirectory || !["connected", "unavailable"].includes(mode)) {
  throw new Error("Usage: node browser-extension-smoke-driver.mjs <profile-directory> <extension-id> <download-directory> <connected|unavailable> [timeout-ms]")
}
const timeoutMilliseconds = Number.parseInt(timeoutArgument, 10)
if (!Number.isFinite(timeoutMilliseconds) || timeoutMilliseconds < 1000) throw new Error("Invalid timeout")

const sleep = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds))
const deadline = Date.now() + timeoutMilliseconds
const directPayload = Buffer.alloc(256 * 1024)
const automaticPayload = Buffer.alloc(256 * 1024)
for (let index = 0; index < directPayload.length; index += 1) {
  directPayload[index] = index % 251
  automaticPayload[index] = (index * 7 + 13) % 251
}
const payloads = new Map([
  ["/fluxdm-browser-smoke.bin", { fileName: "fluxdm-browser-smoke.bin", bytes: directPayload, requests: 0 }],
  ["/fluxdm-browser-auto.bin", { fileName: "fluxdm-browser-auto.bin", bytes: automaticPayload, requests: 0 }],
  ["/fluxdm-browser-fallback.bin", { fileName: "fluxdm-browser-fallback.bin", bytes: automaticPayload, requests: 0 }],
])
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex")

const transferServer = createServer((request, response) => {
  const transfer = payloads.get(request.url?.split("?", 1)[0])
  if (!transfer) {
    response.writeHead(404).end()
    return
  }
  transfer.requests += 1
  const payload = transfer.bytes
  response.setHeader("Accept-Ranges", "bytes")
  response.setHeader("Content-Type", "application/octet-stream")
  response.setHeader("Content-Disposition", `attachment; filename="${transfer.fileName}"`)
  if (request.method === "HEAD") {
    response.setHeader("Content-Length", payload.length)
    response.writeHead(200).end()
    return
  }
  const match = /^bytes=(\d+)-(\d*)$/.exec(request.headers.range || "")
  if (match) {
    const start = Number.parseInt(match[1], 10)
    const end = match[2] ? Math.min(Number.parseInt(match[2], 10), payload.length - 1) : payload.length - 1
    if (start >= payload.length || end < start) {
      response.setHeader("Content-Range", `bytes */${payload.length}`)
      response.writeHead(416).end()
      return
    }
    response.setHeader("Content-Range", `bytes ${start}-${end}/${payload.length}`)
    response.setHeader("Content-Length", end - start + 1)
    response.writeHead(206).end(payload.subarray(start, end + 1))
    return
  }
  response.setHeader("Content-Length", payload.length)
  if (transfer.fileName !== "fluxdm-browser-smoke.bin" && /Chrome|Chromium|Edg/.test(request.headers["user-agent"] || "")) {
    response.writeHead(200)
    let offset = 0
    const timer = setInterval(() => {
      if (response.destroyed || offset >= payload.length) {
        clearInterval(timer)
        if (!response.destroyed) response.end()
        return
      }
      response.write(payload.subarray(offset, Math.min(offset + 8192, payload.length)))
      offset += 8192
    }, 75)
    response.once("close", () => clearInterval(timer))
    return
  }
  response.writeHead(200).end(payload)
})
await new Promise((resolve, reject) => {
  transferServer.once("error", reject)
  transferServer.listen(0, "127.0.0.1", resolve)
})
const transferPort = transferServer.address().port

async function waitForCompletedTransfer(directory, fileName, expectedPayload, owner) {
  const completedPath = join(directory, fileName)
  let completedPayload
  while (Date.now() < deadline) {
    try {
      completedPayload = await readFile(completedPath)
      if (completedPayload.length === expectedPayload.length) break
    } catch {}
    await sleep(50)
  }
  if (!completedPayload || completedPayload.length !== expectedPayload.length) throw new Error(`${owner} did not complete ${fileName} before timeout`)
  const completedSHA256 = sha256(completedPayload)
  const expectedSHA256 = sha256(expectedPayload)
  if (completedSHA256 !== expectedSHA256) throw new Error(`Completed transfer hash mismatch for ${fileName}: ${completedSHA256}`)
  return completedSHA256
}

async function waitForDevToolsPort() {
  let lastError
  while (Date.now() < deadline) {
    try {
      const lines = (await readFile(`${profileDirectory}\\DevToolsActivePort`, "utf8")).trim().split(/\r?\n/)
      const port = Number.parseInt(lines[0], 10)
      if (port > 0 && port <= 65535) return port
    } catch (error) {
      lastError = error
    }
    await sleep(100)
  }
  throw new Error(`Browser did not publish DevToolsActivePort: ${lastError?.message || "timed out"}`)
}

class CDPClient {
  constructor(socket) {
    this.socket = socket
    this.nextID = 1
    this.pending = new Map()
  }

  static async connect(url) {
    const socket = new WebSocket(url)
    const client = new CDPClient(socket)
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("Timed out connecting to the DevTools target")), 5000)
      socket.addEventListener("open", () => {
        clearTimeout(timer)
        resolve()
      }, { once: true })
      socket.addEventListener("error", () => {
        clearTimeout(timer)
        reject(new Error("DevTools WebSocket connection failed"))
      }, { once: true })
    })
    socket.addEventListener("message", async (event) => {
      let raw = event.data
      if (typeof raw !== "string") raw = typeof raw?.text === "function" ? await raw.text() : Buffer.from(raw).toString("utf8")
      const message = JSON.parse(raw)
      if (!message.id) return
      const pending = client.pending.get(message.id)
      if (!pending) return
      client.pending.delete(message.id)
      clearTimeout(pending.timer)
      if (message.error) pending.reject(new Error(`${pending.method}: ${message.error.message}`))
      else pending.resolve(message.result)
    })
    socket.addEventListener("close", () => {
      for (const pending of client.pending.values()) {
        clearTimeout(pending.timer)
        pending.reject(new Error("DevTools target closed"))
      }
      client.pending.clear()
    })
    return client
  }

  call(method, params = {}) {
    const id = this.nextID++
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`${method}: timed out`))
      }, Math.max(1000, deadline - Date.now()))
      this.pending.set(id, { method, resolve, reject, timer })
      this.socket.send(JSON.stringify({ id, method, params }))
    })
  }

  close() {
    this.socket.close()
  }
}

let client
try {
  const port = await waitForDevToolsPort()
  const optionsURL = `chrome-extension://${extensionID}/options.html`
  const targetResponse = await fetch(`http://127.0.0.1:${port}/json/new?${encodeURIComponent(optionsURL)}`, { method: "PUT" })
  if (!targetResponse.ok) throw new Error(`Could not open extension options target: HTTP ${targetResponse.status}`)
  const target = await targetResponse.json()
  if (!target.webSocketDebuggerUrl) throw new Error("Extension options target has no debugger URL")
  client = await CDPClient.connect(target.webSocketDebuggerUrl)
  await client.call("Runtime.enable")
  const expression = `(async () => {
    const deadline = Date.now() + ${Math.max(1000, timeoutMilliseconds - 1000)};
    while (document.readyState !== "complete" && Date.now() < deadline) await new Promise(resolve => setTimeout(resolve, 50));
    const button = document.querySelector("#test");
    const status = document.querySelector("#status");
    if (!button || !status) return { ok: false, status: "extension_page_unavailable", body: document.body?.innerText?.slice(0, 500) || "" };
    button.click();
    while (Date.now() < deadline) {
      const text = status.textContent || "";
      if (text === "Connected to FluxDM") return { ok: true, status: text };
      if (text.startsWith("Not connected")) return { ok: false, status: text };
      await new Promise(resolve => setTimeout(resolve, 50));
    }
    return { ok: false, status: status.textContent || "connection_test_timed_out" };
  })()`
  const evaluation = await client.call("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true })
  if (evaluation.exceptionDetails) throw new Error(`Extension page evaluation failed: ${evaluation.exceptionDetails.text}`)
  const result = evaluation.result?.value
  if (mode === "connected" && !result?.ok) throw new Error(`Extension native connection failed: ${JSON.stringify(result)}`)
  if (mode === "unavailable" && (result?.ok || !result?.status?.startsWith("Not connected"))) {
    throw new Error(`Extension did not report the unavailable desktop: ${JSON.stringify(result)}`)
  }

  let workerTarget
  while (Date.now() < deadline && !workerTarget) {
    const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()
    workerTarget = targets.find((candidate) => candidate.type === "service_worker" && candidate.url === `chrome-extension://${extensionID}/service-worker.js`)
    if (!workerTarget) await sleep(50)
  }
  if (!workerTarget?.webSocketDebuggerUrl) throw new Error("FluxDM extension service worker target was not available")
  const worker = await CDPClient.connect(workerTarget.webSocketDebuggerUrl)
  try {
    await worker.call("Runtime.enable")
    const browserDownloads = join(profileDirectory, "browser-downloads")
    await mkdir(browserDownloads, { recursive: true })
    const version = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json()
    if (!version.webSocketDebuggerUrl) throw new Error("Browser debugger URL was not available")
    const browserClient = await CDPClient.connect(version.webSocketDebuggerUrl)
    try {
      await browserClient.call("Browser.setDownloadBehavior", { behavior: "allow", downloadPath: browserDownloads, eventsEnabled: true })
    } finally {
      browserClient.close()
    }
    await client.call("Page.enable")

    if (mode === "unavailable") {
      const fallbackURL = `http://127.0.0.1:${transferPort}/fluxdm-browser-fallback.bin`
      await client.call("Page.navigate", { url: fallbackURL })
      let fallbackDownload
      while (Date.now() < deadline) {
        const search = await worker.call("Runtime.evaluate", {
          expression: `chrome.downloads.search({}).then(items => items.filter(item => item.url === ${JSON.stringify(fallbackURL)}).map(item => ({ id: item.id, state: item.state, error: item.error || "", exists: item.exists, filename: item.filename })))`,
          awaitPromise: true,
          returnByValue: true,
        })
        fallbackDownload = search.result?.value?.[0]
        if (fallbackDownload?.state === "complete") break
        await sleep(50)
      }
      if (fallbackDownload?.state !== "complete" || fallbackDownload.error) {
        throw new Error(`Browser did not retain the download while FluxDM was unavailable: ${JSON.stringify(fallbackDownload)}`)
      }
      const fallbackPath = resolve(fallbackDownload.filename)
      const insideIsolatedDirectory = [browserDownloads, downloadDirectory].some((directory) => {
        const candidate = relative(resolve(directory), fallbackPath)
        return candidate !== "" ? !candidate.startsWith("..") && !isAbsolute(candidate) : true
      })
      if (!fallbackDownload.filename || !insideIsolatedDirectory) {
        throw new Error(`Browser fallback path escaped its isolated directory: ${fallbackDownload.filename || "missing"}`)
      }
      const fallbackPayload = await readFile(fallbackPath)
      if (fallbackPayload.length !== automaticPayload.length) throw new Error(`Browser fallback length mismatch: ${fallbackPayload.length}`)
      const fallbackSHA256 = sha256(fallbackPayload)
      if (fallbackSHA256 !== sha256(automaticPayload)) throw new Error(`Browser fallback hash mismatch: ${fallbackSHA256}`)
      process.stdout.write(`${JSON.stringify({
        browserPort: port,
        extensionID,
        optionsURL,
        nativeConnection: result.status,
        unavailableFallback: "browser_download_completed",
        fallbackBytes: automaticPayload.length,
        fallbackSHA256,
        fallbackRequests: payloads.get("/fluxdm-browser-fallback.bin").requests,
        browserDownloadState: fallbackDownload.state,
        browserDownloadError: fallbackDownload.error,
      })}\n`)
    } else {
      const transferURL = `http://127.0.0.1:${transferPort}/fluxdm-browser-smoke.bin`
    const handoffEvaluation = await worker.call("Runtime.evaluate", {
      expression: `sendToFluxDM(${JSON.stringify(transferURL)}, "", "fluxdm-browser-smoke.bin", false)`,
      awaitPromise: true,
      returnByValue: true,
    })
    if (handoffEvaluation.exceptionDetails) throw new Error(`Extension handoff evaluation failed: ${handoffEvaluation.exceptionDetails.text}`)
    const handoff = handoffEvaluation.result?.value
    if (!handoff?.accepted) throw new Error(`FluxDM rejected the extension transfer: ${JSON.stringify(handoff)}`)
    const directSHA256 = await waitForCompletedTransfer(downloadDirectory, "fluxdm-browser-smoke.bin", directPayload, "FluxDM")
    const automaticURL = `http://127.0.0.1:${transferPort}/fluxdm-browser-auto.bin`
    await client.call("Page.navigate", { url: automaticURL })
    const automaticSHA256 = await waitForCompletedTransfer(downloadDirectory, "fluxdm-browser-auto.bin", automaticPayload, "FluxDM")

    let browserDownload
    while (Date.now() < deadline) {
      const search = await worker.call("Runtime.evaluate", {
        expression: `chrome.downloads.search({}).then(items => items.filter(item => item.url === ${JSON.stringify(automaticURL)}).map(item => ({ id: item.id, state: item.state, error: item.error || "", exists: item.exists })))`,
        awaitPromise: true,
        returnByValue: true,
      })
      browserDownload = search.result?.value?.[0]
      if (browserDownload?.state === "interrupted") break
      await sleep(50)
    }
    if (browserDownload?.state !== "interrupted" || browserDownload.error !== "USER_CANCELED") {
      throw new Error(`Browser download was not cancelled after FluxDM acceptance: ${JSON.stringify(browserDownload)}`)
    }
    const browserFiles = await readdir(browserDownloads)
    if (browserFiles.includes("fluxdm-browser-auto.bin")) throw new Error("Browser completed its own copy after FluxDM accepted the automatic handoff")

      process.stdout.write(`${JSON.stringify({
      browserPort: port,
      extensionID,
      optionsURL,
      nativeConnection: result.status,
      nativeTransfer: "completed",
      transferBytes: directPayload.length,
      transferSHA256: directSHA256,
      transferRequests: payloads.get("/fluxdm-browser-smoke.bin").requests,
      automaticInterception: "accepted_and_browser_cancelled",
      automaticTransferBytes: automaticPayload.length,
      automaticTransferSHA256: automaticSHA256,
      automaticTransferRequests: payloads.get("/fluxdm-browser-auto.bin").requests,
      browserDownloadState: browserDownload.state,
      browserDownloadError: browserDownload.error,
      })}\n`)
    }
  } finally {
    worker.close()
  }
} finally {
  client?.close()
  transferServer.closeAllConnections()
  await new Promise((resolve) => transferServer.close(resolve))
}
