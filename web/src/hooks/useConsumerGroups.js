import { useApi } from './useApi';
import api from '../api';

/**
 * Hook for fetching and managing consumer groups data
 * @param {Object} options - Configuration options for the useApi hook
 * @returns {Object} - { consumerGroups, loading, error, refreshConsumerGroups }
 */
export const useConsumerGroups = (options = {}) => {
  const { data, loading, error, refresh } = useApi(
    async () => {
      const result = await api.getAllConsumerGroups();
      return result.groups || [];
    },
    [],
    {
      autoRefresh: true,
      refreshInterval: 30000,
      ...options
    }
  );

  return {
    consumerGroups: data || [],
    loading,
    error,
    refreshConsumerGroups: refresh
  };
}
