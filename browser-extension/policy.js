((scope) => {
  const defaultInterceptExtensions = Object.freeze([
    "7z", "apk", "avi", "bz2", "dmg", "exe", "gz", "img", "iso", "m4a", "m4v", "mkv",
    "mov", "mp3", "mp4", "msi", "msix", "ogg", "rar", "tar", "tgz", "wav", "webm", "xz", "zip",
  ])

  function isExcludedHost(value, entries) {
    const host = new URL(value).hostname.toLowerCase()
    return entries.some((entry) => host === String(entry).toLowerCase() || host.endsWith(`.${String(entry).toLowerCase()}`))
  }

  function isExcludedExtension(item, entries) {
    const extension = extensionFor(item.url, item.filename)
    if (!extension) return false
    return entries.some((entry) => extension === String(entry).replace(/^\./, "").toLowerCase())
  }

  function isEligibleLink(url, suggestedFilename, hasDownloadAttribute, entries) {
    if (hasDownloadAttribute) return true
    const extension = extensionFor(url, suggestedFilename)
    return Boolean(extension) && entries.some((entry) => extension === String(entry).replace(/^\./, "").toLowerCase())
  }

  function extensionFor(url, suggestedFilename = "") {
    const candidate = String(suggestedFilename).split(/[\\/]/).pop() || new URL(url).pathname.split("/").pop() || ""
    const dot = candidate.lastIndexOf(".")
    if (dot < 1 || dot === candidate.length - 1) return ""
    return candidate.slice(dot + 1).toLowerCase()
  }

  scope.FluxDMPolicy = Object.freeze({ defaultInterceptExtensions, isExcludedHost, isExcludedExtension, isEligibleLink })
})(globalThis)
