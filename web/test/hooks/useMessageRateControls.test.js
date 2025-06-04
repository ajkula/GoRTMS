import { renderHook, act } from '@testing-library/react';
import { useMessageRateControls } from '../../src/hooks/useMessageRateControls';

describe('useMessageRateControls', () => {
  test('initializes with default values', () => {
    const { result } = renderHook(() => useMessageRateControls());

    expect(result.current.period).toBe('1h');
    expect(result.current.granularity).toBe('auto');
    expect(result.current.isExploring).toBe(false);
  });

  test('changes period and resets granularity to auto', () => {
    const { result } = renderHook(() => useMessageRateControls());

    act(() => {
      result.current.handlePeriodChange('6h');
    });

    expect(result.current.period).toBe('6h');
    expect(result.current.granularity).toBe('auto');
    expect(result.current.isExploring).toBe(true);
  });

  test('resetToDefaults restores initial state', () => {
    const { result } = renderHook(() => useMessageRateControls());

    act(() => {
      result.current.handlePeriodChange('24h');
      result.current.handleGranularityChange('30m');
    });

    act(() => {
      result.current.resetToDefaults();
    });

    expect(result.current.period).toBe('1h');
    expect(result.current.granularity).toBe('auto');
    expect(result.current.isExploring).toBe(false);
  });
});
