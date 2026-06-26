import type { BackendClient, BackendSubscription } from './client'
import type { BootstrapPayload, ChannelUpdateInput, ChannelView, SubscriptionPayload } from '../types'

export class WailsBackendClient implements BackendClient {
  async getBootstrap(): Promise<BootstrapPayload> {
    return window.go.main.FrontendBridge.GetBootstrap()
  }

  subscribe(listener: (payload: SubscriptionPayload) => void): BackendSubscription {
    const eventName = 'procom:state'
    const handler = (payload: SubscriptionPayload) => {
      listener(payload)
    }

    window.runtime.EventsOn(eventName, handler)

    return {
      dispose: () => {
        window.runtime.EventsOff(eventName)
      },
    }
  }

  async updateChannel(input: ChannelUpdateInput): Promise<ChannelView> {
    return window.go.main.FrontendBridge.UpdateChannel(input)
  }
}