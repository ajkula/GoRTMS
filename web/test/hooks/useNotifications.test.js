import { renderHook, waitFor, act } from '@testing-library/react';
import { useNotifications } from '../../src/hooks/useNotifications';
import api from '../../src/api';

// Mock the API module
jest.mock('../../src/api');

describe('useNotifications', () => {
  const mockNotifications = [
    {
      id: '1',
      type: 'warning',
      message: 'Test warning',
      timestamp: Date.now(),
      read: false
    },
    {
      id: '2',
      type: 'info',
      message: 'Test info',
      timestamp: Date.now() - 1000,
      read: true
    }
  ];

  beforeEach(() => {
    jest.clearAllMocks();
    api.getNotifications.mockResolvedValue(mockNotifications);
    api.markNotificationAsRead.mockResolvedValue({ success: true });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('loads notifications on mount', async () => {
    const { result } = renderHook(() => useNotifications());

    // Initial state
    expect(result.current.loading).toBe(true);
    expect(result.current.notifications).toEqual([]);

    // Wait for async update
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.notifications).toEqual(mockNotifications);
    expect(result.current.unreadCount).toBe(1);
    expect(api.getNotifications).toHaveBeenCalledTimes(1);
  });

  test('marks notification as read', async () => {
    const { result } = renderHook(() => useNotifications());

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Mark as read
    await act(async () => {
      await result.current.markAsRead('1');
    });

    expect(result.current.notifications[0].read).toBe(true);
    expect(result.current.unreadCount).toBe(0);
    expect(api.markNotificationAsRead).toHaveBeenCalledWith('1');
  });

  test('handles API errors gracefully', async () => {
    const errorMessage = 'Network error';
    api.getNotifications.mockRejectedValue(new Error(errorMessage));

    const { result } = renderHook(() => useNotifications());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBe(errorMessage);
    });

    expect(result.current.notifications).toEqual([]);
  });

  test('adds local notification', async () => {
    const { result } = renderHook(() => useNotifications());

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Add notification within waitFor
    await act(async () => {
      result.current.addNotification('Local message', 'success');
    });

    expect(result.current.notifications).toHaveLength(3);
    expect(result.current.notifications[0]).toMatchObject({
      message: 'Local message',
      type: 'success',
      local: true
    });
  });

  test('removes local notification', async () => {
    const { result } = renderHook(() => useNotifications());

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Add then remove
    let localId;
    act(() => {
      result.current.addNotification('Temporary', 'info');
      localId = result.current.notifications[0].id;
    });

    act(() => {
      result.current.removeNotification(localId);
    });

    expect(result.current.notifications).toHaveLength(2);
  });

  test('auto-removes local notification after timeout', async () => {
    jest.useFakeTimers();

    const { result } = renderHook(() => useNotifications());

    // Wait for initial load to complete
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
      expect(result.current.notifications).toHaveLength(2); // Initial notifications loaded
    });

    // Add auto-remove notification
    act(() => {
      result.current.addNotification('Temporary', 'success', true);
    });

    // NOW we should have 3 notifications
    expect(result.current.notifications).toHaveLength(3);

    // Advance timers
    act(() => {
      jest.advanceTimersByTime(5000);
    });

    // Back to 2 notifications
    await waitFor(() => {
      expect(result.current.notifications).toHaveLength(2);
    });

    jest.useRealTimers();
  });

  test('refreshes notifications', async () => {
    const { result } = renderHook(() => useNotifications());

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getNotifications).toHaveBeenCalledTimes(1);

    // Refresh
    await act(async () => {
      result.current.refresh();
    });

    expect(api.getNotifications).toHaveBeenCalledTimes(2);
  });

  test('handles mark as read error', async () => {
    const { result } = renderHook(() => useNotifications());

    // Wait for initial load
    await waitFor(() => {

      expect(result.current.loading).toBe(false);
    });

    // Make markAsRead fail
    api.markNotificationAsRead.mockRejectedValue(new Error('Failed to mark'));

    // Try to mark as read
    await act(async () => {
      await result.current.markAsRead('1');
    });

    // Should have called refresh after error
    expect(api.getNotifications).toHaveBeenCalledTimes(2);
  });
});
