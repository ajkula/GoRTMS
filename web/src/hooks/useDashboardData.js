import { useState, useEffect, useCallback } from 'react';
import api from '../api';
import { useMessageRateControls } from './useMessageRateControls';

export function useDashboardData(refreshInterval = 30000) {
  const [data, setData] = useState({
    stats: null,
    resourceHistory: [],
    currentResources: null,
    loading: true,
    error: null
  });

  const messageRateControls = useMessageRateControls();

  const fetchAllData = useCallback(async () => {
    try {
      setData(prev => ({ ...prev, loading: true, error: null }));
      
      const [statsData, historyData, currentData] = await Promise.all([
        api.getStats(messageRateControls.queryParams),
        api.getStatsHistory(30),
        api.getCurrentStats()
      ]);

      const formattedHistory = api.formatHistoryForCharts(historyData);

      setData({
        stats: {
          domains: statsData.domains || 0,
          queues: statsData.queues || 0,
          messages: statsData.messages || 0,
          routes: statsData.routes || 0,
          messageRates: statsData.messageRates || [],
          activeDomains: statsData.activeDomains || [],
          topQueues: statsData.topQueues || [],
          recentEvents: statsData.recentEvents || [],
          queueAlerts: statsData.queueAlerts || [],
          domainTrend: statsData.domainTrend,
          queueTrend: statsData.queueTrend,
          messageTrend: statsData.messageTrend,
          routeTrend: statsData.routeTrend
        },
        resourceHistory: formattedHistory,
        currentResources: currentData,
        loading: false,
        error: null,
        messageRateControls,
      });
    } catch (error) {
      console.error('Error fetching dashboard data:', error);
      setData(prev => ({
        ...prev,
        loading: false,
        error: error.message || 'Failed to load dashboard data'
      }));
    }
  }, [messageRateControls.queryParams]);

  useEffect(() => {
    fetchAllData();
    const interval = setInterval(fetchAllData, refreshInterval);
    return () => clearInterval(interval);
  }, [fetchAllData, refreshInterval]);

  return {
    ...data,
    refresh: fetchAllData
  };
}