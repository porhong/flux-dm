package application

// DownloadRequestEvent is the Wails-facing payload emitted when a browser
// handoff arrives. It carries only the data the UI needs to render a
// confirmation dialog; browser session cookies remain server-side and are
// never included here.
type DownloadRequestEvent struct {
	PendingID         string `json:"pendingId"`
	URL               string `json:"url"`
	SuggestedFilename string `json:"suggestedFilename"`
	Referrer          string `json:"referrer"`
}
