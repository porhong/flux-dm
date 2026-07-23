import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { useUIStore } from "@/stores/ui-store"

import App from "./App"

const { cancelDownloadMock, confirmBrowserDownloadMock, defaultDownloadDirectoryMock, discardBrowserDownloadMock, healthCheckMock, listDownloadsMock, listPendingBrowserDownloadsMock, pauseDownloadMock, probeURLMock, resumeDownloadMock, restartDownloadMock, selectDestinationDirectoryMock, startDownloadMock } = vi.hoisted(() => ({
  cancelDownloadMock: vi.fn(),
  confirmBrowserDownloadMock: vi.fn(),
  defaultDownloadDirectoryMock: vi.fn(),
  discardBrowserDownloadMock: vi.fn(),
  healthCheckMock: vi.fn(),
  listDownloadsMock: vi.fn(),
  listPendingBrowserDownloadsMock: vi.fn(),
  pauseDownloadMock: vi.fn(),
  probeURLMock: vi.fn(),
  resumeDownloadMock: vi.fn(),
  restartDownloadMock: vi.fn(),
  selectDestinationDirectoryMock: vi.fn(),
  startDownloadMock: vi.fn(),
}))

vi.mock("@/lib/backend", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/backend")>()
  return {
    ...actual,
    cancelDownload: cancelDownloadMock,
    confirmBrowserDownload: confirmBrowserDownloadMock,
    defaultDownloadDirectory: defaultDownloadDirectoryMock,
    discardBrowserDownload: discardBrowserDownloadMock,
    healthCheck: healthCheckMock,
    listDownloads: listDownloadsMock,
    listPendingBrowserDownloads: listPendingBrowserDownloadsMock,
    pauseDownload: pauseDownloadMock,
    probeURL: probeURLMock,
    resumeDownload: resumeDownloadMock,
    restartDownload: restartDownloadMock,
    selectDestinationDirectory: selectDestinationDirectoryMock,
    startDownload: startDownloadMock,
  }
})

vi.mock("../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
}))

describe("App", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useUIStore.setState({ activeSection: "downloads", density: "comfortable" })
    listDownloadsMock.mockResolvedValue([])
    listPendingBrowserDownloadsMock.mockResolvedValue([])
    confirmBrowserDownloadMock.mockResolvedValue({ id: "browser-download" })
    defaultDownloadDirectoryMock.mockResolvedValue("C:\\Users\\test\\Downloads")
    discardBrowserDownloadMock.mockResolvedValue(undefined)
    selectDestinationDirectoryMock.mockResolvedValue("")
    healthCheckMock.mockResolvedValue({
      status: "ok",
      version: "test",
      platform: "windows/amd64",
      checkedAt: "2026-01-01T00:00:00Z",
    })
  })

  afterEach(cleanup)

  it("renders the foundation shell and reports backend health", async () => {
    render(<App />)

    expect(screen.getByRole("heading", { name: "Downloads" })).toBeInTheDocument()
    expect(await screen.findByText("Healthy")).toBeInTheDocument()
    expect(healthCheckMock).toHaveBeenCalledOnce()
  })

  it("recovers a browser handoff that arrived before the event listener", async () => {
    probeURLMock.mockResolvedValue({
      url: "https://example.test/archive.zip", finalUrl: "https://example.test/archive.zip", fileName: "archive.zip",
      totalBytes: 2_097_152, mimeType: "application/zip", etag: "etag", lastModified: "", rangeSupported: true, executableWarning: false,
    })
    listPendingBrowserDownloadsMock.mockResolvedValue([{
      pendingId: "browser-pending-1", url: "https://example.test/archive.zip", suggestedFilename: "archive.zip", referrer: "",
    }])

    render(<App />)

    expect(await screen.findByRole("dialog", { name: "Start this download?" })).toBeInTheDocument()
    expect(screen.getByText("archive.zip")).toBeInTheDocument()
  })

  it("passes executable confirmation from a browser handoff to the backend", async () => {
    const user = userEvent.setup()
    probeURLMock.mockResolvedValue({
      url: "https://example.test/setup.exe", finalUrl: "https://example.test/setup.exe", fileName: "setup.exe",
      totalBytes: 2_097_152, mimeType: "application/vnd.microsoft.portable-executable", etag: "etag", lastModified: "", rangeSupported: true, executableWarning: true,
    })
    listPendingBrowserDownloadsMock.mockResolvedValue([{
      pendingId: "browser-pending-exe", url: "https://example.test/setup.exe", suggestedFilename: "setup.exe", referrer: "",
    }])
    startDownloadMock.mockResolvedValue(undefined)

    render(<App />)

    await screen.findByRole("dialog", { name: "Start this download?" })
    await user.click(screen.getByRole("checkbox"))
    await user.click(screen.getByRole("button", { name: "Start download" }))

    await waitFor(() => expect(confirmBrowserDownloadMock).toHaveBeenCalledWith(
      "browser-pending-exe", "C:\\Users\\test\\Downloads", "setup.exe", 4, true,
    ))
    expect(startDownloadMock).toHaveBeenCalledWith("browser-download")
  })

  it("lets a browser handoff choose a destination folder before starting", async () => {
    const user = userEvent.setup()
    probeURLMock.mockResolvedValue({
      url: "https://example.test/archive.zip", finalUrl: "https://example.test/archive.zip", fileName: "archive.zip",
      totalBytes: 2_097_152, mimeType: "application/zip", etag: "etag", lastModified: "", rangeSupported: true, executableWarning: false,
    })
    listPendingBrowserDownloadsMock.mockResolvedValue([{
      pendingId: "browser-pending-folder", url: "https://example.test/archive.zip", suggestedFilename: "archive.zip", referrer: "",
    }])
    selectDestinationDirectoryMock.mockResolvedValue("D:\\Downloads")
    startDownloadMock.mockResolvedValue(undefined)

    render(<App />)

    await screen.findByRole("dialog", { name: "Start this download?" })
    await user.click(screen.getByRole("button", { name: "Browse destination folder" }))
    await waitFor(() => expect(screen.getByRole("textbox", { name: "Destination folder" })).toHaveValue("D:\\Downloads"))
    await user.click(screen.getByRole("button", { name: "Start download" }))

    await waitFor(() => expect(confirmBrowserDownloadMock).toHaveBeenCalledWith(
      "browser-pending-folder", "D:\\Downloads", "archive.zip", 4, false,
    ))
  })

  it("keeps a browser handoff open after inspection fails and discards it only when cancelled", async () => {
    const user = userEvent.setup()
    probeURLMock.mockRejectedValue(new Error("FluxDM could not inspect that URL."))
    listPendingBrowserDownloadsMock.mockResolvedValue([{
      pendingId: "browser-pending-failure", url: "https://example.test/archive.zip", suggestedFilename: "archive.zip", referrer: "",
    }])

    render(<App />)

    await screen.findByRole("dialog", { name: "Start this download?" })
    expect(await screen.findByText("FluxDM could not inspect that URL.")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Start download" })).toBeDisabled()
    await user.click(screen.getByRole("button", { name: "Cancel" }))
    expect(discardBrowserDownloadMock).toHaveBeenCalledWith("browser-pending-failure")
  })

  it("navigates between sidebar sections", async () => {
    const user = userEvent.setup()
    render(<App />)

    await user.click(screen.getByRole("button", { name: "Categories" }))
    expect(screen.getByRole("heading", { name: "Categories" })).toBeInTheDocument()
    expect(screen.getByText("File categories")).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Scheduler" }))
    expect(screen.getByRole("heading", { name: "Scheduler" })).toBeInTheDocument()
    expect(screen.getByText("No schedules yet")).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Settings" }))
    expect(screen.getByRole("heading", { name: "Settings" })).toBeInTheDocument()
    expect(screen.getByText("Interface density")).toBeInTheDocument()
  })

  it("opens the Add Download dialog and rejects unsupported URLs", async () => {
    const user = userEvent.setup()
    render(<App />)

    await user.click(screen.getAllByRole("button", { name: "Add download" })[0])
    expect(screen.getByRole("dialog", { name: "Add download" })).toBeInTheDocument()
    expect(screen.getByRole("combobox", { name: "Connections" })).toHaveTextContent("4 connections")

    await user.type(screen.getByRole("textbox", { name: "Download URL" }), "file:///unsafe.bin")
    await user.click(screen.getByRole("button", { name: "Inspect download" }))
    expect(await screen.findByText("Only HTTP and HTTPS URLs are supported.")).toBeInTheDocument()
  })

  it("shows verified file details before starting a download in the default Downloads folder", async () => {
    const user = userEvent.setup()
    probeURLMock.mockResolvedValue({
      url: "https://example.test/archive.zip", finalUrl: "https://cdn.example.test/archive.zip", fileName: "archive.zip",
      totalBytes: 2_097_152, mimeType: "application/zip", etag: "etag", lastModified: "", rangeSupported: true, executableWarning: false,
    })
    render(<App />)

    await user.click(screen.getAllByRole("button", { name: "Add download" })[0])
    const destination = screen.getByRole("textbox", { name: "Destination folder" })
    await waitFor(() => expect(destination).toHaveValue("C:\\Users\\test\\Downloads"))
    await user.type(screen.getByRole("textbox", { name: "Download URL" }), "https://example.test/archive.zip")
    await user.click(screen.getByRole("button", { name: "Inspect download" }))

    expect(await screen.findByRole("region", { name: "Download details" })).toBeInTheDocument()
    expect(screen.getByText("archive.zip")).toBeInTheDocument()
    expect(screen.getByText("2.00 MB")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Add and start" })).toBeInTheDocument()
  })

  it("offers pause, resume, retry, and restart actions", async () => {
    const user = userEvent.setup()
    listDownloadsMock.mockResolvedValue([
      downloadFixture({ id: "running", fileName: "running.bin", state: "downloading" }),
      downloadFixture({ id: "paused", fileName: "paused.bin", state: "paused", restartRequired: false }),
      downloadFixture({ id: "retry", fileName: "retry.bin", state: "failed", restartRequired: false }),
      downloadFixture({ id: "changed", fileName: "changed.bin", state: "failed", restartRequired: true }),
    ])
    pauseDownloadMock.mockResolvedValue(undefined)
    resumeDownloadMock.mockResolvedValue(undefined)
    startDownloadMock.mockResolvedValue(undefined)
    restartDownloadMock.mockResolvedValue(undefined)
    render(<App />)

    await user.click(await screen.findByRole("button", { name: "Pause running.bin" }))
    await user.click(await screen.findByRole("button", { name: "Resume paused.bin" }))
    await user.click(screen.getByRole("button", { name: "Retry retry.bin" }))
    await user.click(screen.getByRole("button", { name: "Restart changed.bin" }))

    expect(pauseDownloadMock).toHaveBeenCalledWith("running")
    expect(resumeDownloadMock).toHaveBeenCalledWith("paused")
    expect(startDownloadMock).toHaveBeenCalledWith("retry")
    expect(restartDownloadMock).toHaveBeenCalledWith("changed")
  })

  it("virtualizes and searches a 10,000-row download history", async () => {
    const user = userEvent.setup()
    listDownloadsMock.mockResolvedValue(Array.from({ length: 10_000 }, (_, index) => downloadFixture({
      id: `history-${index}`,
      fileName: `archive-${String(index).padStart(5, "0")}.bin`,
      state: "completed",
      downloadedBytes: 1024,
    })))
    render(<App />)

    expect(await screen.findByRole("table", { name: "Downloads" })).toBeInTheDocument()
    expect(screen.getAllByRole("row").length).toBeLessThan(50)
    await user.type(screen.getByRole("textbox", { name: "Search downloads" }), "archive-09999")
    expect(await screen.findByText("archive-09999.bin")).toBeInTheDocument()
    expect(screen.getAllByRole("row").length).toBe(2)
  })

  it("uses a compactable queue layout instead of a fixed-width table", async () => {
    listDownloadsMock.mockResolvedValue([downloadFixture({ id: "compact", fileName: "compact.bin", state: "paused" })])
    render(<App />)

    const table = await screen.findByRole("table", { name: "Downloads" })
    expect(table).toHaveClass("download-list")
    expect(table).not.toHaveClass("min-w-[980px]")
    expect(screen.getByText("compact.bin").closest("[role='row']")).toHaveClass("download-list-row")
  })

  it("supports keyboard selection and properties", async () => {
    const user = userEvent.setup()
    listDownloadsMock.mockResolvedValue([downloadFixture({ id: "keyboard", fileName: "keyboard.bin", state: "completed" })])
    render(<App />)
    const fileName = await screen.findByText("keyboard.bin")
    const row = fileName.closest<HTMLElement>("[role='row']")
    expect(row).not.toBeNull()
    row?.focus()
    await user.keyboard(" ")
    expect(screen.getByText("1 selected")).toBeInTheDocument()
    const selectedRow = screen.getByText("keyboard.bin").closest<HTMLElement>("[role='row']")
    expect(selectedRow).not.toBeNull()
    fireEvent.keyDown(selectedRow as HTMLElement, { key: "Enter" })
    expect(screen.getByRole("dialog", { name: "Download properties" })).toBeInTheDocument()
  })

  it("supports keyboard transfer and global shortcuts", async () => {
    const user = userEvent.setup()
    listDownloadsMock.mockResolvedValue([downloadFixture({ id: "shortcut", fileName: "shortcut.bin", state: "downloading" })])
    pauseDownloadMock.mockResolvedValue(undefined)
    cancelDownloadMock.mockResolvedValue(undefined)
    render(<App />)
    const row = (await screen.findByText("shortcut.bin")).closest<HTMLElement>("[role='row']")
    expect(row).not.toBeNull()
    row?.focus()

    await user.keyboard("p")
    expect(pauseDownloadMock).toHaveBeenCalledWith("shortcut")
    await user.keyboard("{Delete}")
    expect(cancelDownloadMock).toHaveBeenCalledWith("shortcut")
    await user.keyboard("{Control>}a{/Control}")
    expect(screen.getByText("1 selected")).toBeInTheDocument()
    await user.keyboard("{Control>}n{/Control}")
    expect(screen.getByRole("dialog", { name: "Add download" })).toBeInTheDocument()
  })
})

function downloadFixture(overrides: Partial<import("@/lib/backend").DownloadItem>): import("@/lib/backend").DownloadItem {
  return {
    id: "download",
    url: "https://example.test/file.bin",
    finalUrl: "https://example.test/file.bin",
    fileName: "file.bin",
    destinationPath: "C:\\Downloads\\file.bin",
    tempPath: "C:\\Downloads\\file.bin.fluxpart",
    state: "queued",
    totalBytes: 1024,
    downloadedBytes: 256,
    rangeSupported: true,
    restartRequired: false,
    mimeType: "application/octet-stream",
    createdAt: "2026-01-01T00:00:00Z",
    lastError: "",
    retryCount: 0,
    connections: 4,
    segmentCount: 4,
    bandwidthLimit: 0,
	categoryId: "",
	queueId: "",
	queuePosition: 0,
	priority: 0,
	siteProfileId: "",
    ...overrides,
  }
}
