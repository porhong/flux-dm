const assert = require("node:assert/strict")
const test = require("node:test")

require("./policy.js")

const { defaultInterceptExtensions, isEligibleLink, isExcludedHost, isExcludedExtension } = globalThis.FluxDMPolicy

test("host exclusions match the host and its subdomains only", () => {
  assert.equal(isExcludedHost("https://example.com/file.zip", ["example.com"]), true)
  assert.equal(isExcludedHost("https://cdn.example.com/file.zip", ["example.com"]), true)
  assert.equal(isExcludedHost("https://notexample.com/file.zip", ["example.com"]), false)
})

test("extension exclusions use the browser filename or URL path", () => {
  assert.equal(isExcludedExtension({ url: "https://example.com/download", filename: "C:\\Downloads\\image.ISO" }, ["iso"]), true)
  assert.equal(isExcludedExtension({ url: "https://example.com/archive.msi?token=1" }, [".msi"]), true)
  assert.equal(isExcludedExtension({ url: "https://example.com/archive.zip" }, ["iso", "msi"]), false)
  assert.equal(isExcludedExtension({ url: "https://example.com/no-extension" }, ["iso"]), false)
})

test("eligible links need an explicit download attribute or configured file extension", () => {
  assert.equal(isEligibleLink("https://example.com/files/archive.zip", "", false, defaultInterceptExtensions), true)
  assert.equal(isEligibleLink("https://example.com/download", "", true, defaultInterceptExtensions), true)
  assert.equal(isEligibleLink("https://example.com/download", "", false, defaultInterceptExtensions), false)
  assert.equal(isEligibleLink("https://example.com/manual.pdf", "", false, defaultInterceptExtensions), false)
})
