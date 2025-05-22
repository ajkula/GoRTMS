import { useApi } from './useApi';
import api from '../api';

/**
 * Hook for fetching and managing queues for a specific domain
 * @param {string} domainName - The domain name to fetch queues for
 * @param {Object} options - Configuration options for the useApi hook
 * @returns {Object} - { queues, loading, error, refreshQueues }
 */
export function useQueues(domainName, options = {}) {
  const { data: queues, loading, error, refresh } = useApi(
    async () => {
      if (!domainName) return [];
      return await api.getQueues(domainName);
    },
    [domainName],
    {
      autoRefresh: true,
      refreshInterval: 30000,
      ...options
    }
  );

  return { queues: queues || [], loading, error, refreshQueues: refresh };
}
