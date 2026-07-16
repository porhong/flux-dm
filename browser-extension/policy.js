((scope) => {
  function isExcludedHost(value, entries) {
    const host = new URL(value).hostname.toLowerCase()
    return entries.some((entry) => host === String(entry).toLowerCase() || host.endsWith(`.${String(entry).toLowerCase()}`))
  }

  function isExcludedExtension(item, entries) {
    const candidate = item.filename?.split(/[\\/]/).pop() || new URL(item.url).pathname.split("/").pop() || ""
    const dot = candidate.lastIndexOf(".")
    if (dot < 0 || dot === candidate.length - 1) return false
    const extension = candidate.slice(dot + 1).toLowerCase()
    return entries.some((entry) => extension === String(entry).replace(/^\./, "").toLowerCase())
  }

  scope.FluxDMPolicy = Object.freeze({ isExcludedHost, isExcludedExtension })
})(globalThis)
