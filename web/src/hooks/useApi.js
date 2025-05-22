import { useState, useEffect, useCallback } from 'react';

/**
 * Generic hook for API calls with state management
 * @param {Function} apiCall - Function that performs the API call
 * @param {Array} dependencies - Dependencies that trigger the API call when changed
 * @param {Object} options - Configuration options
 * @param {boolean} options.autoRefresh - Enable/disable automatic refresh
 * @param {number} options.refreshInterval - Refresh interval in ms
 * @param {boolean} options.loadOnMount - Load data when component mounts
 * @returns {Object} - { data, loading, error, refresh }
 */
export function useApi(apiCall, dependencies = [], options = {}) {
  const {
    autoRefresh = false,
    refreshInterval = 30000,
    loadOnMount = true,
    initialData = null,
  } = options;

  const [data, setData] = useState(initialData);
  const [loading, setLoading] = useState(loadOnMount);
  const [error, setError] = useState(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await apiCall();
      setData(result);
      return result;
    } catch (err) {
      console.error('Error in API call:', err);
      setError(err.message || 'An error occurred');
      return null;
    } finally {
      setLoading(false);
    }
  }, dependencies);

  useEffect(() => {
    if (loadOnMount) {
      fetchData();
    }

    if (autoRefresh) {
      const interval = setInterval(fetchData, refreshInterval);
      return () => clearInterval(interval);
    }
  }, [fetchData, autoRefresh, refreshInterval, loadOnMount]);

  return { data, loading, error, refresh: fetchData };
}
