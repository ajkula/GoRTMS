import { useState, useEffect, useCallback } from 'react';

export function useNotifications() {
  const [notifications, setNotifications] = useState([]);
  const [unreadCount, setUnreadCount] = useState(0);

  // FAKE for now
  useEffect(() => {
    setNotifications([
      {
        id: '1',
        type: 'warning',
        message: 'Queue orders.processing is approaching capacity',
        timestamp: Date.now() - 5 * 60 * 1000,
        read: false
      },
      {
        id: '2',
        type: 'info',
        message: 'New domain analytics created',
        timestamp: Date.now() - 60 * 60 * 1000,
        read: false
      }
    ]);
    
    setUnreadCount(2);
  }, []);

  const markAsRead = useCallback((notificationId) => {
    setNotifications(prev => 
      prev.map(n => n.id === notificationId ? { ...n, read: true } : n)
    );
    setUnreadCount(prev => Math.max(0, prev - 1));
  }, []);

  return { notifications, unreadCount, markAsRead };
}