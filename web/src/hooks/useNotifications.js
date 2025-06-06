import { useState, useEffect, useCallback } from 'react';
import api from '../api';

export function useNotifications() {
  const [notifications, setNotifications] = useState([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Fetch notifications
  const fetchNotifications = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getNotifications();
      setNotifications(data);
      setUnreadCount(data.filter(n => !n.read).length);
      setError(null);
    } catch (err) {
      setError(err.message || 'Failed to load notifications');
    } finally {
      setLoading(false);
    }
  }, []);

  // Load on mount
  useEffect(() => {
    fetchNotifications();
  }, [fetchNotifications]);

  // Mark as read
  const markAsRead = useCallback(async (notificationId) => {
    try {
      // Optimistic update
      setNotifications(prev => 
        prev.map(n => n.id === notificationId ? { ...n, read: true } : n)
      );
      setUnreadCount(prev => Math.max(0, prev - 1));

      // API call
      await api.markNotificationAsRead(notificationId);
    } catch (err) {
      // Revert on error
      setError(err.message || 'Failed to mark as read');
      await fetchNotifications();
    }
  }, [fetchNotifications]);

  // Add notification (for local notifications like success messages)
  const addNotification = useCallback((message, type = 'info', autoRemove = false) => {
    const notification = {
      id: `local-${Date.now()}`,
      type,
      message,
      timestamp: Date.now(),
      read: false,
      local: true // Flag to distinguish from API notifications
    };

    setNotifications(prev => [notification, ...prev]);
    setUnreadCount(prev => prev + 1);

    // Auto-remove after 5 seconds if requested
    if (autoRemove) {
      setTimeout(() => {
        removeNotification(notification.id);
      }, 5000);
    }
  }, []);

  // Remove notification (for local notifications)
  const removeNotification = useCallback((notificationId) => {
    setNotifications(prev => {
      const notification = prev.find(n => n.id === notificationId);
      if (notification && !notification.read) {
        setUnreadCount(count => Math.max(0, count - 1));
      }
      return prev.filter(n => n.id !== notificationId);
    });
  }, []);

  return { 
    notifications, 
    unreadCount, 
    loading,
    error,
    markAsRead,
    addNotification,
    removeNotification,
    refresh: fetchNotifications
  };
}
