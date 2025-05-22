
import { useState, useCallback } from 'react';
import api from '../api';

/**
 * Hook for managing consumer group CRUD operations
 * @param {Function} refreshCallback - Function to call after successful operations
 * @returns {Object} - { createConsumerGroup, deleteConsumerGroup, loading, error }
 */
export function useConsumerGroupActions(refreshCallback) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const createConsumerGroup = useCallback(
    async (domainName, queueName, groupData) => {
      setLoading(true);
      setError(null);
      try {
        const result = await api.createConsumerGroup(domainName, queueName, groupData);
        if (refreshCallback) {
          await refreshCallback();
        }
        return result;
      } catch (err) {
        console.error('Error creating consumer group:', err);
        setError(err.message || 'Failed to create consumer group');
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [refreshCallback]
  );

  const deleteConsumerGroup = useCallback(
    async (domainName, queueName, groupID) => {
      setLoading(true);
      setError(null);
      try {
        const result = await api.deleteConsumerGroup(domainName, queueName, groupID);
        if (refreshCallback) {
          await refreshCallback();
        }
        return result;
      } catch (err) {
        console.error('Error deleting consumer group:', err);
        setError(err.message || 'Failed to delete consumer group');
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [refreshCallback]
  );

  return {
    createConsumerGroup,
    deleteConsumerGroup,
    loading,
    error
  };
}

// Example of combined usage in a component
// const ConsumerGroupManager = () => {
//   const { consumerGroups, loading, error, refreshConsumerGroups } = useConsumerGroups();
//   const { createConsumerGroup, deleteConsumerGroup, loading: actionLoading, error: actionError } = 
//     useConsumerGroupActions(refreshConsumerGroups);
//   
//   // ... rest of the component
// };
