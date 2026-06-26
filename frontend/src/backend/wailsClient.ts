import type { BackendClient, BackendSubscription } from './client'
import type { BootstrapPayload, ChannelAddInput, ChannelUpdateInput, ChannelView, KeywordRuleInput, OscSettingsInput, SubscriptionPayload } from '../types'

export class WailsBackendClient implements BackendClient {
  async getBootstrap(): Promise<BootstrapPayload> {
    return window.go.main.FrontendBridge.GetBootstrap()
  }

  subscribe(listener: (payload: SubscriptionPayload) => void): BackendSubscription {
    const eventName = 'intercom:state'
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

  async addChannel(input: ChannelAddInput): Promise<ChannelView> {
    return (window.go.main.FrontendBridge as any).AddChannel(input)
  }

  async removeChannel(channelId: string): Promise<void> {
    await (window.go.main.FrontendBridge as any).RemoveChannel(channelId)
  }

  async updateKeywords(rules: KeywordRuleInput[]): Promise<void> {
    await (window.go.main.FrontendBridge as any).UpdateKeywords(rules)
  }

  async updateOsc(input: OscSettingsInput): Promise<void> {
    await (window.go.main.FrontendBridge as any).UpdateOSC(input)
  }
}