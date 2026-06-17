'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  type CloudApiConfig,
  getDefaultCloudApiConfig,
  getStoredCloudApiConfig,
  saveStoredCloudApiConfig,
} from '@/lib/api';

export function useApiConfig() {
  const [config, setConfig] = useState<CloudApiConfig>(() => getDefaultCloudApiConfig());
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const hydrate = () => {
      setConfig(getStoredCloudApiConfig());
      setReady(true);
    };

    const hydrationTimer = window.setTimeout(hydrate, 0);

    const syncFromStorage = () => {
      setConfig(getStoredCloudApiConfig());
    };

    window.addEventListener('storage', syncFromStorage);
    return () => {
      window.clearTimeout(hydrationTimer);
      window.removeEventListener('storage', syncFromStorage);
    };
  }, []);

  const updateConfig = useCallback((update: Partial<CloudApiConfig>) => {
    const next = saveStoredCloudApiConfig(update);
    setConfig(next);
    return next;
  }, []);

  const hasCredentials = useMemo(() => Boolean(config.apiKey), [config.apiKey]);

  return {
    config,
    ready,
    hasCredentials,
    updateConfig,
  };
}
