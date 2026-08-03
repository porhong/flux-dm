const handoffToggle = document.querySelector("#handoff-toggle")
const handoffDescription = document.querySelector("#handoff-description")
const connectionStatus = document.querySelector("#connection-status")
const connectionDot = document.querySelector("#connection-dot")
const message = document.querySelector("#message")
const refresh = document.querySelector("#refresh")
const testConnection = document.querySelector("#test-connection")
let handoffEnabled = true
function setMessage(value, isError = false) { message.textContent = value; message.classList.toggle("is-error", isError) }
function renderHandoff() { handoffToggle.setAttribute("aria-checked", String(handoffEnabled)); handoffDescription.textContent = handoffEnabled ? "Eligible downloads go to FluxDM." : "Downloads stay in your browser." }
function setConnection(response) { const connected = response?.accepted === true; connectionStatus.textContent = connected ? "Connected to FluxDM" : "Desktop app unavailable"; connectionDot.classList.toggle("is-connected", connected); connectionDot.classList.toggle("is-offline", !connected); return connected }
function connectionFailureMessage(response) { if (response?.code !== "native_error") return "Open FluxDM, then try again."; const detail = String(response.message || "").toLowerCase(); if (detail.includes("not found")) return "The FluxDM native host is not registered. Open FluxDM Settings and choose Repair integration, then restart your browser."; if (detail.includes("exited")) return "The FluxDM native host could not start. In FluxDM Settings, choose Repair integration and confirm the portable ZIP is fully extracted."; return "Browser integration needs repair. In FluxDM Settings, choose Repair integration, then restart your browser." }
async function checkConnection({ announce = false } = {}) { refresh.disabled = true; testConnection.disabled = true; connectionStatus.textContent = "Checking connection"; connectionDot.classList.remove("is-connected", "is-offline"); try { const response = await chrome.runtime.sendMessage({ type: "fluxdm:ping" }); const connected = setConnection(response); if (announce) setMessage(connected ? "FluxDM is ready for downloads." : connectionFailureMessage(response), !connected) } catch { setConnection(null); if (announce) setMessage("Browser integration is unavailable. Open FluxDM Settings, choose Repair integration, then restart your browser.", true) } finally { refresh.disabled = false; testConnection.disabled = false } }
chrome.storage.sync.get({ enabled: true }).then((settings) => { handoffEnabled = settings.enabled; renderHandoff() })
handoffToggle.addEventListener("click", async () => { handoffEnabled = !handoffEnabled; renderHandoff(); await chrome.storage.sync.set({ enabled: handoffEnabled }); setMessage(handoffEnabled ? "Link handoff is on." : "Link handoff is paused.") })
refresh.addEventListener("click", () => void checkConnection({ announce: true }))
testConnection.addEventListener("click", () => void checkConnection({ announce: true }))
document.querySelector("#open-settings").addEventListener("click", () => chrome.runtime.openOptionsPage())
void checkConnection()
