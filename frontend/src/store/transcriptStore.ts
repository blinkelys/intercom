import { create } from 'zustand'
import type { BackendClient, BackendSubscription } from '../backend/client'
import type { AudioLevelsView, ChannelAddInput, ChannelUpdateInput, ChannelView, InputDeviceView, KeywordRuleInput, KeywordRuleView, OscSettingsInput, OscSettingsView, RuntimeStatus, SpeechDiagnosticsView, TranscriptEntryView, TranscriptPartialView } from '../types'

const defaultSpeechDiagnostics: SpeechDiagnosticsView = {
  model: 'mlx-community/whisper-tiny',
  task: 'transcribe',
  temperature: 0,
  bestOf: 5,
  beamSize: 5,
  channelLanguages: {},
  lastChannelId: '',
  lastLanguage: '',
  lastInferenceMs: 0,
  lastTextChars: 0,
  lastError: '',
  updatedAt: '',
}

const defaultOscSettings: OscSettingsView = {
  enabled: false,
  destination: '',
  port: 0,
}

type TranscriptStore = {
  channels: ChannelView[]
  inputDevices: InputDeviceView[]
  audioLevels: AudioLevelsView
  entries: TranscriptEntryView[]
  partials: Record<string, TranscriptPartialView>
  keywords: KeywordRuleView[]
  osc: OscSettingsView
  speech: SpeechDiagnosticsView
  selectedChannelId: string
  status: RuntimeStatus
  engineLabel: string
  keywordCount: number
  initialized: boolean
  savingChannel: boolean
  bootstrap: (client: BackendClient) => Promise<() => void>
  selectChannel: (channelId: string) => void
  saveChannel: (client: BackendClient, input: ChannelUpdateInput) => Promise<void>
  addChannel: (client: BackendClient, input: ChannelAddInput) => Promise<void>
  removeChannel: (client: BackendClient, channelId: string) => Promise<void>
  saveKeywords: (client: BackendClient, rules: KeywordRuleInput[]) => Promise<void>
  saveOsc: (client: BackendClient, input: OscSettingsInput) => Promise<void>
}

let activeSubscription: BackendSubscription | null = null

export const useTranscriptStore = create<TranscriptStore>((set) => ({
  channels: [],
  inputDevices: [],
  audioLevels: {},
  entries: [],
  partials: {},
  keywords: [],
  osc: defaultOscSettings,
  speech: defaultSpeechDiagnostics,
  selectedChannelId: 'all',
  status: 'connecting',
  engineLabel: 'Pending',
  keywordCount: 0,
  initialized: false,
  savingChannel: false,
  bootstrap: async (client) => {
    const payload = await client.getBootstrap()

    set({
      channels: payload.channels,
      inputDevices: payload.inputDevices,
      audioLevels: payload.audioLevels,
      entries: payload.snapshot.entries,
      partials: payload.snapshot.partials,
      keywords: payload.keywords,
      osc: payload.osc,
      speech: payload.speech,
      status: payload.status,
      engineLabel: payload.engineLabel,
      keywordCount: payload.keywordCount,
      initialized: true,
    })

    activeSubscription?.dispose()
    activeSubscription = client.subscribe((update) => {
      set((state) => ({
        channels: update.channels ?? state.channels,
        inputDevices: update.inputDevices ?? state.inputDevices,
        audioLevels: update.audioLevels ?? state.audioLevels,
        entries: update.snapshot?.entries ?? state.entries,
        partials: update.snapshot?.partials ?? state.partials,
        keywords: update.keywords ?? state.keywords,
        osc: update.osc ?? state.osc,
        speech: update.speech ?? state.speech,
        status: update.status ?? state.status,
      }))
    })

    return () => {
      activeSubscription?.dispose()
      activeSubscription = null
    }
  },
  selectChannel: (channelId) => {
    set({ selectedChannelId: channelId })
  },
  saveChannel: async (client, input) => {
    set({ savingChannel: true })
    try {
      const updated = await client.updateChannel(input)
      set((state) => ({
        channels: state.channels.map((channel) => (channel.id === updated.id ? updated : channel)),
        savingChannel: false,
      }))
    } catch (error) {
      set({ savingChannel: false, status: 'offline' })
      throw error
    }
  },
  addChannel: async (client, input) => {
    set({ savingChannel: true })
    try {
      const added = await client.addChannel(input)
      set((state) => ({
        channels: [...state.channels, added],
        selectedChannelId: added.id,
        savingChannel: false,
      }))
    } catch (error) {
      set({ savingChannel: false, status: 'offline' })
      throw error
    }
  },
  removeChannel: async (client, channelId) => {
    set({ savingChannel: true })
    try {
      await client.removeChannel(channelId)
      set((state) => ({
        channels: state.channels.filter((channel) => channel.id !== channelId),
        entries: state.entries.filter((entry) => entry.channelId !== channelId),
        partials: Object.fromEntries(Object.entries(state.partials).filter(([id]) => id !== channelId)),
        selectedChannelId: state.selectedChannelId === channelId ? 'all' : state.selectedChannelId,
        savingChannel: false,
      }))
    } catch (error) {
      set({ savingChannel: false, status: 'offline' })
      throw error
    }
  },
  saveKeywords: async (client, rules) => {
    set({ savingChannel: true })
    try {
      await client.updateKeywords(rules)
      set({ keywords: rules, savingChannel: false })
    } catch (error) {
      set({ savingChannel: false, status: 'offline' })
      throw error
    }
  },
  saveOsc: async (client, input) => {
    set({ savingChannel: true })
    try {
      await client.updateOsc(input)
      set({ osc: input, savingChannel: false })
    } catch (error) {
      set({ savingChannel: false, status: 'offline' })
      throw error
    }
  },
}))