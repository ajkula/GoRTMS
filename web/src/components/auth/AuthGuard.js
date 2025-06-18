import React from 'react';
import { useAuth } from '../../hooks/useAuth';

export const AuthGuard = ({ 
  children, 
  requiredRole = null,
  fallback = <div className="p-4 text-center text-gray-500">Access denied</div> 
}) => {
  const { isAuthenticated, hasRole, loading } = useAuth();

  if (loading) {
    return <div className="p-4 text-center">Loading...</div>;
  }

  if (!isAuthenticated) {
    return <div className="p-4 text-center text-red-500">Please login to access this page</div>;
  }

  if (requiredRole && !hasRole(requiredRole)) {
    return fallback;
  }

  return children;
};
