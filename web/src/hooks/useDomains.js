import { useApi } from './useApi';
import api from '../api';

export const useDomains = (options = {}) => {
  const { data: domains, loading, error, refresh } = useApi(
    () => api.getDomains(),
    [],
    {
      autoRefresh: true,
      refreshInterval: 30000,
      ...options
    }
  );

  return { domains: domains || [], loading, error, refreshDomains: refresh };
}
