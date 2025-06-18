import { useState, useEffect, useCallback } from 'react';
import { authService } from '../services/authService';

let globalAuthState = {
  user: null,
  isAuthenticated: false,
  loading: true,
  error: null,
  initialized: false
};

let subscribers = new Set();

const notifySubscribers = () => {
  subscribers.forEach(callback => callback(globalAuthState));
};

const initializeAuth = async () => {
  if (globalAuthState.initialized) return;
  
  try {
    const currentUser = await authService.getCurrentUser();
    globalAuthState = {
      user: currentUser,
      isAuthenticated: !!currentUser,
      loading: false,
      error: null,
      initialized: true
    };
  } catch (err) {
    globalAuthState = {
      user: null,
      isAuthenticated: false,
      loading: false,
      error: err,
      initialized: true
    };
    authService.removeToken();
  }
  notifySubscribers();
};

export const useAuth = () => {
  const [state, setState] = useState(globalAuthState);

  useEffect(() => {
    const handleStateChange = (newState) => {
      setState({ ...newState });
    };
    
    subscribers.add(handleStateChange);
    
    if (!globalAuthState.initialized) {
      initializeAuth();
    }
    
    return () => subscribers.delete(handleStateChange);
  }, []);

  const login = useCallback(async (username, password) => {
    try {
      const result = await authService.login(username, password);
      globalAuthState = {
        user: result.user,
        isAuthenticated: true,
        loading: false,
        error: null,
        initialized: true
      };
      notifySubscribers();
      return result;
    } catch (error) {
      globalAuthState = {
        user: null,
        isAuthenticated: false,
        loading: false,
        error,
        initialized: true
      };
      notifySubscribers();
      throw error;
    }
  }, []);

  const logout = useCallback(() => {
    authService.logout();
    globalAuthState = {
      user: null,
      isAuthenticated: false,
      loading: false,
      error: null,
      initialized: true
    };
    notifySubscribers();
  }, []);

  const refresh = useCallback(async () => {
    globalAuthState = { ...globalAuthState, loading: true };
    notifySubscribers();
    
    try {
      const currentUser = await authService.getCurrentUser();
      globalAuthState = {
        user: currentUser,
        isAuthenticated: !!currentUser,
        loading: false,
        error: null,
        initialized: true
      };
    } catch (err) {
      globalAuthState = {
        user: null,
        isAuthenticated: false,
        loading: false,
        error: err,
        initialized: true
      };
    }
    notifySubscribers();
  }, []);

  const hasRole = useCallback((requiredRole) => {
    return authService.hasRole(state.user, requiredRole);
  }, [state.user]);

  const isAdmin = useCallback(() => hasRole('admin'), [hasRole]);
  const isUser = useCallback(() => hasRole('user'), [hasRole]);

  return {
    user: state.user,
    isAuthenticated: state.isAuthenticated,
    loading: state.loading,
    error: state.error,
    login,
    logout,
    hasRole,
    isAdmin,
    isUser,
    refresh,
  };
};

export const forceLogout = () => {
  globalAuthState = {
    user: null,
    isAuthenticated: false,
    loading: false,
    error: null,
    initialized: true
  };
  notifySubscribers();
};
