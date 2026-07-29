import React from "react"
import { createRoot } from "react-dom/client"

import App from "./App"
import { BrowserConfirmationSurface } from "./features/downloads/browser-confirmation-surface"
import "./style.css"

const container = document.getElementById('root')

if (!container) throw new Error("FluxDM root element was not found")

const root = createRoot(container)

const isBrowserConfirmation = new URLSearchParams(window.location.search).get("surface") === "browser-confirm"

root.render(
  <React.StrictMode>
    {isBrowserConfirmation ? <BrowserConfirmationSurface /> : <App />}
  </React.StrictMode>,
)
