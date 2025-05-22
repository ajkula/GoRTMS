import { useApi } from './useApi';
import api from '../api';

/**
 * Hook for fetching and managing dashboard statistics
 * @param {Object} options - Configuration options for the useApi hook
 * @returns {Object} - { stats, loading, error, refreshStats }
 */
export function useDashboardStats(options = {}) {
  const { data, loading, error, refresh } = useApi(
    async () => {
      // Fetch dashboard statistics
      const statsData = await api.getStats();
      
      // Add default properties if they don't exist to prevent errors in components
      return {
        domains: statsData.domains || 0,
        queues: statsData.queues || 0,
        messages: statsData.messages || 0,
        routes: statsData.routes || 0,
        messageRates: statsData.messageRates || [],
        activeDomains: statsData.activeDomains || [],
        topQueues: statsData.topQueues || [],
        recentEvents: statsData.recentEvents || []
      };
    },
    [],
    {
      autoRefresh: true,
      refreshInterval: 30000, // Match the current refresh interval in Dashboard.js
      ...options
    }
  );

  return {
    stats: data,
    loading,
    error,
    refreshStats: refresh
  };
}
