import { useEffect } from 'react'
import type { BackendClient } from '../backend/client'
import { useTranscriptStore } from '../store/transcriptStore'

export function useTranscriptRuntime(client: BackendClient) {
  const bootstrap = useTranscriptStore((state) => state.bootstrap)

  useEffect(() => {
    let dispose: (() => void) | undefined
    let cancelled = false

    bootstrap(client)
      .then((cleanup) => {
        if (cancelled) {
          cleanup()
          return
        }
        dispose = cleanup
      })
      .catch(() => {
        useTranscriptStore.setState({ status: 'offline', initialized: true })
      })

    return () => {
      cancelled = true
      dispose?.()
    }
  }, [bootstrap, client])
}