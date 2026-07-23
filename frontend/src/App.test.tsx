import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { useUIStore } from "@/stores/ui-store"

import App from "./App"

const { cancelDownloadMock, healthCheckMock, listDownloadsMock, openCompletedDownloadFileMock, pauseDownloadMock, recycleCompletedDownloadFilesMock, resumeDownloadMock, restartDownloadMock, startDownloadMock } = vi.hoisted(() => ({
  cancelDownloadMock: vi.fn(),
  healthCheckMock: vi.fn(),
  listDownloadsMock: vi.fn(),
  openCompletedDownloadFileMock: vi.fn(),
  pauseDownloadMock: vi.fn(),
  recycleCompletedDownloadFilesMock: vi.fn(),
  resumeDownloadMock: vi.fn(),
  restartDownloadMock: vi.fn(),
  startDownloadMock: vi.fn(),
}))

vi.mock("@/lib/backend", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/backend")>()
  return {
    ...actual,
    cancelDownload: cancelDownloadMock,
    healthCheck: healthCheckMock,
    listDownloads: listDownloadsMock,
    openCompletedDownloadFile: openCompletedDownloadFileMock,
    pauseDownload: pauseDownloadMock,
    resumeDownload: resumeDownloadMock,
    restartDownload: restartDownloadMock,
    recycleCompletedDownloadFiles: recycleCompletedDownloadFilesMock,
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
    await user.click(screen.getByRole("button", { name: "Add and start" }))
    expect(await screen.findByText("Only HTTP and HTTPS URLs are supported.")).toBeInTheDocument()
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

  it("opens a completed file only after an explicit row action", async () => {
    const user = userEvent.setup()
    listDownloadsMock.mockResolvedValue([downloadFixture({ id: "completed", fileName: "report.pdf", state: "completed" })])
    openCompletedDownloadFileMock.mockResolvedValue(undefined)
    render(<App />)

    await user.click(await screen.findByRole("button", { name: "More actions for report.pdf" }))
    await user.click(screen.getByRole("menuitem", { name: /open/i }))
    expect(openCompletedDownloadFileMock).toHaveBeenCalledWith("completed")
  })

  it("confirms recycling selected completed files", async () => {
    const user = userEvent.setup()
    listDownloadsMock.mockResolvedValue([downloadFixture({ id: "completed", fileName: "report.pdf", state: "completed" })])
    recycleCompletedDownloadFilesMock.mockResolvedValue({ updated: [], removedIds: ["completed"], skippedIds: [], failures: [] })
    render(<App />)

    await user.click(await screen.findByLabelText("Select report.pdf"))
    await user.click(screen.getByRole("button", { name: "Remove files" }))
    expect(screen.getByRole("dialog", { name: "Remove completed files" })).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Recycle files" }))
    expect(recycleCompletedDownloadFilesMock).toHaveBeenCalledWith(["completed"])
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
