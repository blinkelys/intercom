import type { BackendClient, BackendSubscription } from './client'
import type {
  BootstrapPayload,
  ChannelView,
  ChannelUpdateInput,
  InputDeviceView,
  SpeechDiagnosticsView,
  SubscriptionPayload,
  TranscriptEntryView,
  TranscriptPartialView,
  TranscriptSnapshotView,
} from '../types'

const initialChannels: ChannelView[] = [
  { id: 'producer', name: 'Producer', color: '#ef4444', icon: '🎬', inputDevice: 'Input 1', language: 'en', enabled: true },
  { id: 'musical-director', name: 'Musical Director', color: '#22c55e', icon: '🎼', inputDevice: 'Input 2', language: 'en', enabled: true },
]

const initialInputDevices: InputDeviceView[] = [
  { id: 'Input 1', name: 'Input 1' },
  { id: 'Input 2', name: 'Input 2' },
]

const initialSpeech: SpeechDiagnosticsView = {
  model: 'mlx-community/whisper-tiny',
  task: 'transcribe',
  temperature: 0,
  bestOf: 5,
  beamSize: 5,
  channelLanguages: {
    producer: 'en',
    'musical-director': 'en',
  },
  lastChannelId: 'producer',
  lastLanguage: 'en',
  lastInferenceMs: 640,
  lastTextChars: 36,
  lastError: '',
  updatedAt: '19:42:10',
}

type FeedStep = {
  channelId: string
  partial: string
  final: string
}

const feed: FeedStep[] = [
  {
    channelId: 'producer',
    partial: 'Stand by on automation for deck track.',
    final: 'Stand by on automation for deck track at cue twelve.',
  },
  {
    channelId: 'musical-director',
    partial: 'Hold the downbeat and wait for the clear.',
    final: 'Hold the downbeat and wait for the clear from stage management.',
  },
  {
    channelId: 'producer',
    partial: 'Go for stage left crossover on my mark.',
    final: 'Go for stage left crossover on my mark after the applause dies.',
  },
]

export class MockBackendClient implements BackendClient {
  private listeners = new Set<(payload: SubscriptionPayload) => void>()
  private channels = [...initialChannels]
  private snapshot: TranscriptSnapshotView = this.createInitialSnapshot()
  private nextEntry = 103
  private timer: number | null = null
  private stepIndex = 0

  async getBootstrap(): Promise<BootstrapPayload> {
    this.startFeed()

    return {
      channels: this.channels,
      inputDevices: initialInputDevices,
      audioLevels: {},
      snapshot: this.snapshot,
      speech: initialSpeech,
      status: 'live',
      engineLabel: 'Offline worker bridge',
      keywordCount: 3,
    }
  }

  subscribe(listener: (payload: SubscriptionPayload) => void): BackendSubscription {
    this.listeners.add(listener)
    listener({ channels: this.channels, inputDevices: initialInputDevices, audioLevels: {}, snapshot: this.snapshot, speech: initialSpeech, status: 'live' })

    return {
      dispose: () => {
        this.listeners.delete(listener)
      },
    }
  }

  async updateChannel(input: ChannelUpdateInput): Promise<ChannelView> {
    const nextChannel: ChannelView = {
      id: input.id,
      name: input.name.trim(),
      color: input.color.trim(),
      icon: input.icon,
      inputDevice: input.inputDevice.trim(),
      language: input.language.trim(),
      enabled: input.enabled,
    }

    this.channels = this.channels.map((channel) => (channel.id === nextChannel.id ? nextChannel : channel))
    this.snapshot = {
      entries: this.snapshot.entries.map((entry) =>
        entry.channelId === nextChannel.id
          ? {
              ...entry,
              channelName: nextChannel.name,
              color: nextChannel.color,
              icon: nextChannel.icon,
            }
          : entry,
      ),
      partials: Object.fromEntries(
        Object.entries(this.snapshot.partials).map(([channelId, partial]) => [
          channelId,
          channelId === nextChannel.id
            ? {
                ...partial,
                channelName: nextChannel.name,
                color: nextChannel.color,
                icon: nextChannel.icon,
              }
            : partial,
        ]),
      ),
    }

    this.emit()
    return nextChannel
  }

  private startFeed() {
    if (this.timer !== null) {
      return
    }

    this.timer = window.setInterval(() => {
      const step = feed[this.stepIndex % feed.length]
      const channel = this.channels.find((item) => item.id === step.channelId)
      if (!channel) {
        return
      }

      const partial: TranscriptPartialView = {
        channelId: channel.id,
        channelName: channel.name,
        color: channel.color,
        icon: channel.icon,
        timestamp: formatTime(new Date()),
        text: step.partial,
      }

      this.snapshot = {
        entries: this.snapshot.entries,
        partials: {
          ...this.snapshot.partials,
          [channel.id]: partial,
        },
      }
      this.emit()

      window.setTimeout(() => {
        const entry: TranscriptEntryView = {
          id: `tx-${String(this.nextEntry++).padStart(6, '0')}`,
          channelId: channel.id,
          channelName: channel.name,
          color: channel.color,
          icon: channel.icon,
          timestamp: formatTime(new Date()),
          text: step.final,
          keywords: inferKeywords(step.final),
          highlights: inferHighlights(step.final),
          finalized: true,
        }

        const partials = { ...this.snapshot.partials }
        delete partials[channel.id]

        this.snapshot = {
          entries: [...this.snapshot.entries, entry],
          partials,
        }
        this.emit()
      }, 1400)

      this.stepIndex += 1
    }, 3800)
  }

  private emit() {
    for (const listener of this.listeners) {
      listener({ channels: this.channels, inputDevices: initialInputDevices, audioLevels: {}, snapshot: this.snapshot, speech: initialSpeech, status: 'live' })
    }
  }

  private createInitialSnapshot(): TranscriptSnapshotView {
    return {
      entries: [
        {
          id: 'tx-000101',
          channelId: 'producer',
          channelName: 'Producer',
          color: '#ef4444',
          icon: '🎬',
          timestamp: '19:42:10',
          text: 'Standby for automation check at stage left.',
          keywords: ['Standby'],
          highlights: [{ phrase: 'Standby', color: '#22c55e', start: 0, end: 7 }],
          finalized: true,
        },
        {
          id: 'tx-000102',
          channelId: 'musical-director',
          channelName: 'Musical Director',
          color: '#22c55e',
          icon: '🎼',
          timestamp: '19:42:14',
          text: 'Go on the intro vamp after the next bar.',
          keywords: ['Go'],
          highlights: [{ phrase: 'Go', color: '#eab308', start: 0, end: 2 }],
          finalized: true,
        },
      ],
      partials: {},
    }
  }
}

function formatTime(date: Date): string {
  return new Intl.DateTimeFormat('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date)
}

function inferKeywords(text: string): string[] {
  const keywords = ['Go', 'Stand by', 'clear']
  return keywords.filter((phrase) => text.toLowerCase().includes(phrase.toLowerCase()))
}

function inferHighlights(text: string) {
  return inferKeywords(text)
    .map((phrase) => {
      const lowerText = text.toLowerCase()
      const lowerPhrase = phrase.toLowerCase()
      const startByte = lowerText.indexOf(lowerPhrase)
      if (startByte < 0) {
        return null
      }

      const start = Array.from(text.slice(0, startByte)).length
      const end = start + Array.from(text.slice(startByte, startByte + phrase.length)).length

      return {
        phrase,
        color: phrase.toLowerCase() === 'go' ? '#eab308' : phrase.toLowerCase() === 'clear' ? '#f97316' : '#22c55e',
        start,
        end,
      }
    })
    .filter((value): value is NonNullable<typeof value> => value !== null)
}