import type { BootstrapPayload, ChannelAddInput, ChannelUpdateInput, ChannelView, KeywordRuleInput, OscSettingsInput, SubscriptionPayload } from '../types'

export type BackendSubscription = {
  dispose: () => void
}

export interface BackendClient {
  getBootstrap(): Promise<BootstrapPayload>
  subscribe(listener: (payload: SubscriptionPayload) => void): BackendSubscription
  updateChannel(input: ChannelUpdateInput): Promise<ChannelView>
  addChannel(input: ChannelAddInput): Promise<ChannelView>
  removeChannel(channelId: string): Promise<void>
  updateKeywords(rules: KeywordRuleInput[]): Promise<void>
  updateOsc(input: OscSettingsInput): Promise<void>
}