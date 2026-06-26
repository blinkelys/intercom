import type { BackendClient } from './client'
import { MockBackendClient } from './mockClient'
import { WailsBackendClient } from './wailsClient'

export function createBackendClient(): BackendClient {
  if (hasWailsRuntime()) {
    return new WailsBackendClient()
  }
  return new MockBackendClient()
}

function hasWailsRuntime(): boolean {
  return typeof window !== 'undefined'
    && typeof window.go !== 'undefined'
    && typeof window.go.main !== 'undefined'
    && typeof window.go.main.FrontendBridge !== 'undefined'
    && typeof window.go.main.FrontendBridge.GetBootstrap === 'function'
    && typeof window.go.main.FrontendBridge.UpdateChannel === 'function'
    && typeof window.runtime !== 'undefined'
    && typeof window.runtime.EventsOn === 'function'
}