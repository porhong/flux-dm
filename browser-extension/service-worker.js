importScripts("policy.js")

const HOST = "com.fluxdm.browser"
const MAX_URL_LENGTH = 8192
const { isExcludedHost, isExcludedExtension } = globalThis.FluxDMPolicy

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({ id: "fluxdm-download", title: "Download with FluxDM", contexts: ["link"] })
})

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId !== "fluxdm-download" || !info.linkUrl) return
  void sendToFluxDM(info.linkUrl, tab?.url || "", "")
})

chrome.downloads.onCreated.addListener((item) => {
  void interceptDownload(item)
})

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type !== "fluxdm:ping") return false
  sendNative({ version: 1, requestId: requestID(), type: "ping" }).then(sendResponse)
  return true
})

async function interceptDownload(item) {
  if (!item.url || !isHTTP(item.url) || item.url.length > MAX_URL_LENGTH) return
  const settings = await chrome.storage.sync.get({ enabled: true, excludedHosts: [], excludedExtensions: [], shareCookies: false })
  if (!settings.enabled || isExcludedHost(item.url, settings.excludedHosts) || isExcludedExtension(item, settings.excludedExtensions)) return
  const response = await sendToFluxDM(item.url, item.referrer || "", item.filename?.split(/[\\/]/).pop() || "", settings.shareCookies)
  // The browser remains responsible for the download unless FluxDM explicitly accepted it.
  if (response?.accepted) await chrome.downloads.cancel(item.id)
}

async function sendToFluxDM(url, referrer, suggestedFilename, shareCookies = false) {
  if (!isHTTP(url) || url.length > MAX_URL_LENGTH) return { accepted: false, code: "invalid_url" }
  let cookies = ""
  if (shareCookies) cookies = (await chrome.cookies.getAll({ url })).map(cookie => `${cookie.name}=${cookie.value}`).join("; ")
  return sendNative({ version: 1, requestId: requestID(), type: "add", url, referrer: isHTTP(referrer) ? referrer : "", suggestedFilename, cookies })
}

function sendNative(message) {
  return new Promise((resolve) => chrome.runtime.sendNativeMessage(HOST, message, (response) => {
    if (chrome.runtime.lastError) resolve({ accepted: false, code: "native_error", message: chrome.runtime.lastError.message })
    else resolve(response || { accepted: false, code: "empty_response" })
  }))
}
function isHTTP(value) { try { const url = new URL(value); return url.protocol === "http:" || url.protocol === "https:" } catch { return false } }
function requestID() { return `${Date.now().toString(36)}-${crypto.randomUUID().replaceAll("-", "")}` }
