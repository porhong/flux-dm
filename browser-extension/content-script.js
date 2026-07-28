const { defaultInterceptExtensions, isEligibleLink, isExcludedExtension, isExcludedHost } = globalThis.FluxDMPolicy
let settings = { enabled: true, excludedHosts: [], excludedExtensions: [], interceptExtensions: defaultInterceptExtensions }

void chrome.storage.sync.get(settings).then((stored) => { settings = stored })
chrome.storage.onChanged.addListener((changes, areaName) => {
  if (areaName !== "sync") return
  for (const [key, change] of Object.entries(changes)) settings[key] = change.newValue
})

document.addEventListener("click", (event) => {
  if (!event.isTrusted || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return

  const link = event.target instanceof Element ? event.target.closest("a[href]") : null
  if (!link || link.target && link.target !== "_self") return

  let url
  try {
    url = new URL(link.href)
  } catch {
    return
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") return

  const suggestedFilename = link.getAttribute("download") || ""
  if (!settings.enabled || isExcludedHost(url.href, settings.excludedHosts) || isExcludedExtension({ url: url.href, filename: suggestedFilename }, settings.excludedExtensions) || !isEligibleLink(url.href, suggestedFilename, link.hasAttribute("download"), settings.interceptExtensions)) return
  event.preventDefault()
  event.stopImmediatePropagation()

  void handOff(url.href, suggestedFilename, link.hasAttribute("download"))
}, true)

async function handOff(url, suggestedFilename, hasDownloadAttribute) {
  const response = await sendMessage({ type: "fluxdm:handoff-link", url, referrer: location.href, suggestedFilename, hasDownloadAttribute })
  if (response?.accepted) return

  // Preserve the browser as a safe fallback when FluxDM is unavailable.
  location.assign(url)
}

function sendMessage(message) {
  return new Promise((resolve) => chrome.runtime.sendMessage(message, (response) => {
    if (chrome.runtime.lastError) resolve({ accepted: false })
    else resolve(response || { accepted: false })
  }))
}
