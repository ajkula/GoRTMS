import { renderHook, act, waitFor } from '@testing-library/react';
import { useHealthCheck } from '../../src/hooks/useHealthCheck';
import api from '../../src/api';

jest.mock('../../src/api');

describe('useHealthCheck', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers();
    jest.spyOn(console, 'error').mockImplementation(() => { });
  });

  afterEach(() => {
    console.error.mockRestore();
    jest.runOnlyPendingTimers();
    jest.useRealTimers();
  });

  test('calls healthCheck on mount and sets systemHealthy=true for ok', async () => {
    api.healthCheck.mockResolvedValue({ status: 'ok' });

    const { result } = renderHook(() => useHealthCheck(5000));

    expect(result.current.loading).toBe(true);
    await act(async () => {
      await waitFor(() => {
        expect(result.current.systemHealthy).toBe(true);
      });
    });

    expect(api.healthCheck).toHaveBeenCalledTimes(1);
  });

  test('sets systemHealthy=false on API error', async () => {
    api.healthCheck.mockRejectedValue(new Error('fail'));

    const { result } = renderHook(() => useHealthCheck(5000));

    await waitFor(() => {
      expect(result.current.systemHealthy).toBe(false);
    });

    expect(api.healthCheck).toHaveBeenCalledTimes(1);
  });

  test('calls healthCheck periodically', async () => {
    api.healthCheck.mockResolvedValue({ status: 'ok' });

    renderHook(() => useHealthCheck(5000));

    expect(api.healthCheck).toHaveBeenCalledTimes(1);

    act(() => {
      jest.advanceTimersByTime(5000);
    });

    await waitFor(() => {
      expect(api.healthCheck).toHaveBeenCalledTimes(2);
    });

    act(() => {
      jest.advanceTimersByTime(5000);
    });

    await waitFor(() => {
      expect(api.healthCheck).toHaveBeenCalledTimes(3);
    });
  });
});
