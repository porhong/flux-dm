import { describe, expect, it } from "vitest"

import { downloadSchema } from "./backend"

describe("downloadSchema", () => {
  it("accepts queued downloads with unset lifecycle timestamps", () => {
    const result = downloadSchema.safeParse({
      id: "download-1",
      url: "https://example.test/file.bin",
      finalUrl: "",
      fileName: "file.bin",
      destinationPath: "C:\\Downloads\\file.bin",
      tempPath: "C:\\Downloads\\file.bin.fluxpart",
      state: "queued",
      totalBytes: -1,
      downloadedBytes: 0,
      rangeSupported: false,
      restartRequired: false,
      mimeType: "",
      createdAt: "2026-07-16T00:00:00Z",
      startedAt: null,
      completedAt: null,
      lastError: "",
      retryCount: 0,
      connections: 4,
      segmentCount: 1,
      bandwidthLimit: 0,
      categoryId: "",
      queueId: "",
      queuePosition: 1,
      priority: 0,
      siteProfileId: "",
    })

    expect(result.success).toBe(true)
  })
})
