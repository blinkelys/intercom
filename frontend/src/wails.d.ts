import type { BootstrapPayload, ChannelUpdateInput, ChannelView, SubscriptionPayload } from './types'

declare global {
  interface Window {
    go: {
      main: {
        FrontendBridge: {
          GetBootstrap: () => Promise<BootstrapPayload>
          UpdateChannel: (input: ChannelUpdateInput) => Promise<ChannelView>
        }
      }
    }
    runtime: {
      EventsOn: (eventName: string, callback: (payload: SubscriptionPayload) => void) => void
      EventsOff: (eventName: string) => void
    }
  }
}

export {}