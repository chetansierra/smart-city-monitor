import { useEffect, useRef } from 'react';
import { API_BASE_URL } from '../constants/app';
import type { StreamMessage } from '../types/app';

type SSEListener = (message: StreamMessage) => void;

/** Singleton shared EventSource keyed by session ID. */
const connections = new Map<string, { source: EventSource; listeners: Set<SSEListener>; refCount: number }>();

function getOrCreateConnection(sessionId: string) {
  let entry = connections.get(sessionId);
  if (entry) {
    entry.refCount++;
    return entry;
  }

  const streamUrl = API_BASE_URL.replace(/\/api\/v1\/?$/, '/stream');
  const url = new URL(streamUrl, window.location.origin);
  url.searchParams.set('session_id', sessionId);

  const source = new EventSource(url.toString());
  const listeners = new Set<SSEListener>();

  source.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data) as StreamMessage;
      for (const cb of listeners) {
        cb(payload);
      }
    } catch {
      // ignore malformed stream payload
    }
  };

  entry = { source, listeners, refCount: 1 };
  connections.set(sessionId, entry);
  return entry;
}

function releaseConnection(sessionId: string) {
  const entry = connections.get(sessionId);
  if (!entry) return;
  entry.refCount--;
  if (entry.refCount <= 0) {
    entry.source.close();
    connections.delete(sessionId);
  }
}

/**
 * Subscribe to the shared SSE connection for a session.
 * Multiple hooks can share the same underlying EventSource.
 */
export function useSSEConnection(sessionId: string, enabled: boolean, onMessage: SSEListener) {
  const callbackRef = useRef(onMessage);
  callbackRef.current = onMessage;

  useEffect(() => {
    if (!enabled || !sessionId) return;

    const entry = getOrCreateConnection(sessionId);
    const handler: SSEListener = (msg) => callbackRef.current(msg);
    entry.listeners.add(handler);

    return () => {
      entry.listeners.delete(handler);
      releaseConnection(sessionId);
    };
  }, [enabled, sessionId]);
}
