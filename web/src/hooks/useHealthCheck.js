import { useState, useEffect } from 'react';
import api from '../api';

export function useHealthCheck(checkInterval = 30000) {
  const [systemHealthy, setSystemHealthy] = useState(true);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const checkHealth = async () => {
      try {
        const health = await api.healthCheck();
        setSystemHealthy(health.status === 'ok');
      } catch (err) {
        console.error('Health check failed:', err);
        setSystemHealthy(false);
      } finally {
        setLoading(false);
      }
    };

    checkHealth();
    const interval = setInterval(checkHealth, checkInterval);
    return () => clearInterval(interval);
  }, [checkInterval]);

  return { systemHealthy, loading };
}
