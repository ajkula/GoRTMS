import { useState, useEffect, useCallback } from 'react';
import api from '../api';

export const useAccountRequests = (filter = 'pending', autoRefresh = true) => {
  const [requests, setRequests] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchRequests = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      
      const filterParam = filter !== 'all' ? filter : null;
      const data = await api.getAccountRequests(filterParam);
      setRequests(data.requests || []);
    } catch (err) {
      console.error('Error fetching account requests:', err);
      setError(err.message || 'Failed to load account requests');
    } finally {
      setLoading(false);
    }
  }, [filter]);

  useEffect(() => {
    fetchRequests();

    // Auto refresh every 30 seconds for pending requests
    if (autoRefresh && filter === 'pending') {
      const interval = setInterval(fetchRequests, 30000);
      return () => clearInterval(interval);
    }
  }, [fetchRequests, autoRefresh, filter]);

  const createRequest = useCallback(async (requestData) => {
    try {
      const result = await api.createAccountRequest(requestData);
      // Refresh the list if we're showing pending requests
      if (filter === 'pending' || filter === 'all') {
        fetchRequests();
      }
      return result;
    } catch (err) {
      console.error('Error creating account request:', err);
      throw err;
    }
  }, [filter, fetchRequests]);

  const reviewRequest = useCallback(async (requestId, reviewData) => {
    try {
      const result = await api.reviewAccountRequest(requestId, reviewData);
      // Refresh the list
      fetchRequests();
      return result;
    } catch (err) {
      console.error('Error reviewing account request:', err);
      throw err;
    }
  }, [fetchRequests]);

  const deleteRequest = useCallback(async (requestId) => {
    try {
      const result = await api.deleteAccountRequest(requestId);
      // Refresh the list
      fetchRequests();
      return result;
    } catch (err) {
      console.error('Error deleting account request:', err);
      throw err;
    }
  }, [fetchRequests]);

  const approveRequest = useCallback(async (requestId, approvedRole = null) => {
    return reviewRequest(requestId, {
      approve: true,
      ...(approvedRole && { approvedRole })
    });
  }, [reviewRequest]);

  const rejectRequest = useCallback(async (requestId, rejectReason = '') => {
    return reviewRequest(requestId, {
      approve: false,
      rejectReason
    });
  }, [reviewRequest]);

  // Helper to get pending requests count
  const pendingCount = requests.filter(req => req.status === 'pending').length;

  return {
    requests,
    loading,
    error,
    pendingCount,
    refresh: fetchRequests,
    createRequest,
    reviewRequest,
    deleteRequest,
    approveRequest,
    rejectRequest
  };
};
