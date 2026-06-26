import type { BootstrapPayload, ChannelUpdateInput, ChannelView, SubscriptionPayload } from '../types'

export type BackendSubscription = {
  dispose: () => void
}

export interface BackendClient {
  getBootstrap(): Promise<BootstrapPayload>
  subscribe(listener: (payload: SubscriptionPayload) => void): BackendSubscription
  updateChannel(input: ChannelUpdateInput): Promise<ChannelView>
}