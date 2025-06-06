import { useState, useCallback, useMemo } from 'react';

const PERIODS = [
  { value: '1h', label: '1 Hour', default: true },
  { value: '6h', label: '6 Hours' },
  { value: '12h', label: '12 Hours' },
  { value: '24h', label: '24 Hours' }
];

const GRANULARITIES = {
  '1h': [
    { value: 'auto', label: 'Auto (1m)', default: true },
    { value: '10s', label: '10 seconds' },
    { value: '1m', label: '1 minute' }
  ],
  '6h': [
    { value: 'auto', label: 'Auto (5m)', default: true },
    { value: '1m', label: '1 minute' },
    { value: '5m', label: '5 minutes' },
    { value: '15m', label: '15 minutes' }
  ],
  '12h': [
    { value: 'auto', label: 'Auto (15m)', default: true },
    { value: '5m', label: '5 minutes' },
    { value: '15m', label: '15 minutes' },
    { value: '30m', label: '30 minutes' }
  ],
  '24h': [
    { value: 'auto', label: 'Auto (30m)', default: true },
    { value: '15m', label: '15 minutes' },
    { value: '30m', label: '30 minutes' },
    { value: '1h', label: '1 hour' }
  ]
};

export const useMessageRateControls = () => {
  const [period, setPeriod] = useState('1h');
  const [granularity, setGranularity] = useState('auto');
  const [isExploring, setIsExploring] = useState(false);

  const availableGranularities = useMemo(() => {
    return GRANULARITIES[period] || GRANULARITIES['1h'];
  }, [period]);

  const handlePeriodChange = useCallback((newPeriod) => {
    setPeriod(newPeriod);
    setGranularity('auto'); // Reset to auto when period changes
    setIsExploring(newPeriod !== '1h');
  }, []);

  const handleGranularityChange = useCallback((newGranularity) => {
    setGranularity(newGranularity);
    setIsExploring(!(newGranularity === 'auto' && period === '1h'));
    // setIsExploring(!(['1m', 'auto'].includes(newGranularity) && period === '1h'));
  }, []);

  const resetToDefaults = useCallback(() => {
    setPeriod('1h');
    setGranularity('auto');
    setIsExploring(false);
  }, []);

  const queryParams = useMemo(() => ({
    period,
    granularity
  }), [period, granularity]);

  return {
    period,
    granularity,
    isExploring,
    periods: PERIODS,
    availableGranularities,
    queryParams,
    handlePeriodChange,
    handleGranularityChange,
    resetToDefaults
  };
}
