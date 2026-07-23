importScripts("policy.js")

const HOST = "com.fluxdm.browser"
const MAX_URL_LENGTH = 8192
const { defaultInterceptExtensions, isEligibleLink, isExcludedHost, isExcludedExtension } = globalThis.FluxDMPolicy

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({ id: "fluxdm-download", title: "Download with FluxDM", contexts: ["link"] })
})

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId !== "fluxdm-download" || !info.linkUrl) return
  void sendToFluxDM(info.linkUrl, tab?.url || "", "")
})

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type === "fluxdm:ping") {
    sendNative({ version: 1, requestId: requestID(), type: "ping" }).then(sendResponse)
    return true
  }
  if (message?.type === "fluxdm:handoff-link") {
    void handOffLink(message).then(sendResponse)
    return true
  }
  return false
})

async function handOffLink(message) {
  if (typeof message.url !== "string" || typeof message.referrer !== "string" || typeof message.suggestedFilename !== "string" || typeof message.hasDownloadAttribute !== "boolean") return { accepted: false, code: "invalid_message" }
  if (!isHTTP(message.url) || message.url.length > MAX_URL_LENGTH) return { accepted: false, code: "invalid_url" }

  const settings = await chrome.storage.sync.get({ enabled: true, excludedHosts: [], excludedExtensions: [], interceptExtensions: defaultInterceptExtensions, shareCookies: false })
  if (!settings.enabled || isExcludedHost(message.url, settings.excludedHosts) || isExcludedExtension({ url: message.url, filename: message.suggestedFilename }, settings.excludedExtensions)) return { accepted: false, code: "excluded" }
  if (!isEligibleLink(message.url, message.suggestedFilename, message.hasDownloadAttribute, settings.interceptExtensions)) return { accepted: false, code: "not_eligible" }

  return sendToFluxDM(message.url, message.referrer, message.suggestedFilename, settings.shareCookies)
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
