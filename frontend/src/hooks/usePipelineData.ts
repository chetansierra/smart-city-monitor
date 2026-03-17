import { useCallback, useEffect, useRef, useState } from 'react';
import { API_BASE_URL } from '../constants/app';
import type { PipelineFlowStats } from '../types/app';

interface Params {
  activeNav: string;
}

export function usePipelineData({ activeNav }: Params) {
  const [pipelineStats, setPipelineStats] = useState<PipelineFlowStats | null>(null);
  const [pipelineLoading, setPipelineLoading] = useState(false);
  const [pipelineError, setPipelineError] = useState('');

  const isPipeline = activeNav === 'nerds';
  const fetchRef = useRef<(() => Promise<void>) | null>(null);

  useEffect(() => {
    if (!isPipeline) return;

    let cancelled = false;

    const fetchStats = async () => {
      if (!cancelled && !pipelineStats) setPipelineLoading(true);
      try {
        const response = await fetch(`${API_BASE_URL}/pipeline/flow/stats`, {
          cache: 'no-store',
        });
        if (!response.ok) throw new Error('Failed to fetch pipeline stats');
        const payload = await response.json();
        if (!cancelled) {
          setPipelineStats(payload?.data ?? null);
          setPipelineError('');
        }
      } catch {
        if (!cancelled) setPipelineError('Could not load pipeline stats.');
      } finally {
        if (!cancelled) setPipelineLoading(false);
      }
    };

    fetchRef.current = fetchStats;

    setPipelineError('');
    void fetchStats();
    const timer = window.setInterval(fetchStats, 5000);

    const refreshOnVisible = () => {
      if (document.visibilityState !== 'visible') return;
      void fetchStats();
    };
    window.addEventListener('focus', refreshOnVisible);
    document.addEventListener('visibilitychange', refreshOnVisible);

    return () => {
      cancelled = true;
      fetchRef.current = null;
      window.clearInterval(timer);
      window.removeEventListener('focus', refreshOnVisible);
      document.removeEventListener('visibilitychange', refreshOnVisible);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isPipeline]);

  const refreshPipeline = useCallback(() => {
    if (fetchRef.current) void fetchRef.current();
  }, []);

  return { pipelineStats, pipelineLoading, pipelineError, refreshPipeline };
}
