import { renderHook, waitFor } from '@testing-library/react';
import { useDomains } from '../../src/hooks/useDomains';
import api from '../../src/api';
import { act } from 'react';

// Mock the API module
jest.mock('../../src/api');

describe('useDomains', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.spyOn(console, 'error').mockImplementation(() => { });
  });

  afterEach(() => {
    console.error.mockRestore();
  });

  test('fetches domains on mount', async () => {
    const mockDomains = [
      { name: 'domain1', queues: {} },
      { name: 'domain2', queues: {} }
    ];

    api.getDomains.mockResolvedValue(mockDomains);

    const { result } = renderHook(() => useDomains());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.domains).toEqual(mockDomains);
    expect(api.getDomains).toHaveBeenCalledTimes(1);
  });

  test('fetches domains on mount', async () => {
    const mockDomains = [
      { name: 'domain1', queues: {} },
      { name: 'domain2', queues: {} }
    ];

    api.getDomains.mockResolvedValue(mockDomains);

    const { result } = renderHook(() => useDomains());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.domains).toEqual(mockDomains);
    expect(api.getDomains).toHaveBeenCalledTimes(1);
  });

  test('handles fetch error', async () => {
    const errorMessage = 'Failed to fetch domains';
    api.getDomains.mockRejectedValue(new Error(errorMessage));

    const { result } = renderHook(() => useDomains());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe(errorMessage);
    expect(result.current.domains).toEqual([]);
  });

  // test.only('creates new domain', async () => {
  //   const initialDomains = [{ name: 'domain1', queues: {} }];
  //   const newDomain = { name: 'domain2', queues: {} };

  //   api.getDomains.mockResolvedValue(initialDomains);
  //   api.createDomain.mockResolvedValue(newDomain);

  //   const { result } = renderHook(() => useDomains());

  //   await waitFor(() => {
  //     expect(result.current.loading).toBe(false);
  //   });

  //   await act(async () => {
  //     await api.createDomain('domain2');
  //   });

  //   expect(api.createDomain).toHaveBeenCalledWith('domain2');
  //   expect(api.getDomains).toHaveBeenCalledTimes(2); // Initial + refetch
  // });

  // test('deletes domain', async () => {
  //   const domains = [
  //     { name: 'domain1', queues: {} },
  //     { name: 'domain2', queues: {} }
  //   ];

  //   api.getDomains.mockResolvedValue(domains);
  //   api.deleteDomain.mockResolvedValue({});

  //   const { result } = renderHook(() => useDomains());

  //   await waitFor(() => {
  //     expect(result.current.loading).toBe(false);
  //   });

  //   await waitFor(async () => {
  //     await result.current.deleteDomain('domain1');
  //   });

  //   expect(api.deleteDomain).toHaveBeenCalledWith('domain1');
  //   expect(api.getDomains).toHaveBeenCalledTimes(2); // Initial + refetch
  // });
});
