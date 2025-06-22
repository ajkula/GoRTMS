import { renderHook, waitFor } from '@testing-library/react';
import { useServices } from '../../src/hooks/useServices';
import api from '../../src/api';
import { act } from 'react';

jest.mock('../../src/api');

describe('useServices', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.spyOn(console, 'error').mockImplementation(() => { });
    // Mock setTimeout pour éviter les warnings de test
    jest.useFakeTimers();
  });

  afterEach(() => {
    console.error.mockRestore();
    jest.runOnlyPendingTimers();
    jest.useRealTimers();
  });

  test('fetches services on mount', async () => {
    const mockServices = [
      {
        id: 'service-1',
        name: 'test-service',
        permissions: ['publish:orders'],
        enabled: true,
        lastUsed: '2024-01-01T10:00:00Z'
      },
      {
        id: 'service-2',
        name: 'another-service',
        permissions: ['consume:*'],
        enabled: false,
        lastUsed: '2024-01-01T10:00:00Z'
      }
    ];

    api.getServices.mockResolvedValue(mockServices);

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.services).toEqual(mockServices);
    expect(api.getServices).toHaveBeenCalledTimes(1);
  });

  test('handles fetch error', async () => {
    const errorMessage = 'Failed to load services';
    api.getServices.mockRejectedValue(new Error(errorMessage));

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe(errorMessage);
    expect(result.current.services).toEqual([]);
  });

  test('creates service successfully', async () => {
    const mockServices = [];
    const newService = {
      name: 'new-service',
      permissions: ['publish:orders'],
      ipWhitelist: []
    };
    const createdService = {
      id: 'new-service-123',
      secret: 'secret123',
      ...newService,
      enabled: true
    };

    api.getServices.mockResolvedValue(mockServices);
    api.createService.mockResolvedValue(createdService);

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.createService(newService);
    });

    expect(api.createService).toHaveBeenCalledWith(newService);
    expect(api.getServices).toHaveBeenCalledTimes(2); // Initial + refetch
    expect(result.current.message).toContain('🔑 NEW SECRET (save now!): secret123');
    expect(result.current.message).toContain('SECRET');
  });

  test('handles create service error', async () => {
    const mockServices = [];
    const errorMessage = 'Service already exists';

    api.getServices.mockResolvedValue(mockServices);
    api.createService.mockRejectedValue(new Error(errorMessage));

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      try {
        await result.current.createService({ name: 'test', permissions: ['*'] });
      } catch (err) {
        // Expected to throw
      }
    });

    expect(result.current.error).toBe(errorMessage);
  });

  test('deletes service successfully', async () => {
    const mockServices = [{ id: 'service-1', name: 'test-service' }];

    api.getServices.mockResolvedValue(mockServices);
    api.deleteService.mockResolvedValue({});

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.deleteService('service-1', 'test-service');
    });

    expect(api.deleteService).toHaveBeenCalledWith('service-1');
    expect(api.getServices).toHaveBeenCalledTimes(2);
    expect(result.current.message).toContain('test-service');
    expect(result.current.message).toContain('deleted');
  });

  test('rotates secret successfully', async () => {
    const mockServices = [{ id: 'service-1' }];
    const rotateResponse = { secret: 'new-secret-123' };

    api.getServices.mockResolvedValue(mockServices);
    api.rotateServiceSecret.mockResolvedValue(rotateResponse);

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.rotateSecret('service-1');
    });

    expect(api.rotateServiceSecret).toHaveBeenCalledWith('service-1');
    expect(result.current.message).toContain('new-secret-123');
  });

  test('updates permissions successfully', async () => {
    const mockServices = [{ id: 'service-1' }];
    const permissions = {
      permissions: ['consume:*'],
      ipWhitelist: ['192.168.1.*'],
      enabled: false
    };

    api.getServices.mockResolvedValue(mockServices);
    api.updateServicePermissions.mockResolvedValue({});

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.updatePermissions('service-1', permissions);
    });

    expect(api.updateServicePermissions).toHaveBeenCalledWith('service-1', permissions);
    expect(result.current.message).toContain('updated successfully');
  });

  test('copies to clipboard', async () => {
    // Mock clipboard API
    const mockWriteText = jest.fn().mockResolvedValue(undefined);

    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: mockWriteText,
      },
      writable: true,
    });

    api.getServices.mockResolvedValue([]);

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.copyToClipboard('test-text');
    });

    expect(mockWriteText).toHaveBeenCalledWith('test-text');
    expect(result.current.message).toBe('Copied to clipboard!');
  });

  test('clears messages after timeout', async () => {
    api.getServices.mockResolvedValue([]);

    const { result } = renderHook(() => useServices());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.copyToClipboard('test');
    });

    expect(result.current.message).toBe('Copied to clipboard!');

    act(() => {
      jest.advanceTimersByTime(5000);
    });

    expect(result.current.message).toBe('');
  });
});
