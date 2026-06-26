export type ChannelView = {
  id: string
  name: string
  color: string
  icon: string
  inputDevice: string
  language: string
  enabled: boolean
}

export type InputDeviceView = {
  id: string
  name: string
}

export type AudioLevelsView = Record<string, number>

export type ChannelUpdateInput = {
  id: string
  name: string
  color: string
  icon: string
  inputDevice: string
  language: string
  enabled: boolean
}

export type TranscriptEntryView = {
  id: string
  channelId: string
  channelName: string
  color: string
  icon: string
  timestamp: string
  text: string
  keywords: string[]
  highlights: TranscriptHighlightView[]
  finalized: true
}

export type TranscriptHighlightView = {
  phrase: string
  color: string
  start: number
  end: number
}

export type TranscriptPartialView = {
  channelId: string
  channelName: string
  color: string
  icon: string
  timestamp: string
  text: string
}

export type TranscriptSnapshotView = {
  entries: TranscriptEntryView[]
  partials: Record<string, TranscriptPartialView>
}

export type SpeechDiagnosticsView = {
  model: string
  task: string
  temperature: number
  bestOf: number
  beamSize: number
  channelLanguages: Record<string, string>
  lastChannelId: string
  lastLanguage: string
  lastInferenceMs: number
  lastTextChars: number
  lastError: string
  updatedAt: string
}

export type RuntimeStatus = 'connecting' | 'live' | 'offline'

export type BootstrapPayload = {
  channels: ChannelView[]
  inputDevices: InputDeviceView[]
  audioLevels: AudioLevelsView
  snapshot: TranscriptSnapshotView
  speech: SpeechDiagnosticsView
  status: RuntimeStatus
  engineLabel: string
  keywordCount: number
}

export type SubscriptionPayload = {
  channels?: ChannelView[]
  inputDevices?: InputDeviceView[]
  audioLevels?: AudioLevelsView
  snapshot?: TranscriptSnapshotView
  speech?: SpeechDiagnosticsView
  status?: RuntimeStatus
}