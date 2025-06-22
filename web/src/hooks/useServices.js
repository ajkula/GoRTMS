import { useState, useEffect, useCallback } from 'react';
import api from '../api';

export const useServices = () => {
  const [services, setServices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');

  const clearMessages = useCallback(() => {
    setMessage('');
    setError('');
  }, []);

  const showMessage = useCallback((msg, isError = false) => {
    if (isError) {
      setError(msg);
      setMessage('');
    } else {
      setMessage(msg);
      setError('');
    }
    setTimeout(() => {
      setMessage('');
      setError('');
    }, 5000);
  }, []);

  const loadServices = useCallback(async () => {
    try {
      setLoading(true);
      const servicesList = await api.getServices();
      setServices(servicesList);
    } catch (err) {
      showMessage(err.message || 'Failed to load services', true);
    } finally {
      setLoading(false);
    }
  }, [showMessage]);

  const createService = useCallback(async (serviceData) => {
    try {
      const response = await api.createService(serviceData);
      showMessage(`Service '${serviceData.name}' created successfully!`);
      
      // Show secret if returned
      if (response.secret) {
        setMessage(`🔑 NEW SECRET (save now!): ${response.secret}`);
      }
      
      await loadServices();
      return response;
    } catch (err) {
      showMessage(err.message || 'Failed to create service', true);
      throw err;
    }
  }, [showMessage, loadServices]);

  const deleteService = useCallback(async (serviceId, serviceName) => {
    try {
      await api.deleteService(serviceId);
      showMessage(`Service "${serviceName}" deleted successfully!`);
      await loadServices();
    } catch (err) {
      showMessage(err.message || 'Failed to delete service', true);
      throw err;
    }
  }, [showMessage, loadServices]);

  const rotateSecret = useCallback(async (serviceId) => {
    try {
      const response = await api.rotateServiceSecret(serviceId);
      showMessage(`🔑 NEW SECRET (save now!): ${response.secret}`);
      await loadServices();
      return response;
    } catch (err) {
      showMessage(err.message || 'Failed to rotate secret', true);
      throw err;
    }
  }, [showMessage, loadServices]);

  const updatePermissions = useCallback(async (serviceId, permissions) => {
    try {
      await api.updateServicePermissions(serviceId, permissions);
      showMessage('Service permissions updated successfully!');
      await loadServices();
    } catch (err) {
      showMessage(err.message || 'Failed to update permissions', true);
      throw err;
    }
  }, [showMessage, loadServices]);

  const copyToClipboard = useCallback((text) => {
    navigator.clipboard.writeText(text);
    showMessage('Copied to clipboard!');
  }, [showMessage]);

  useEffect(() => {
    loadServices();
  }, [loadServices]);

  return {
    services,
    loading,
    message,
    error,
    clearMessages,
    loadServices,
    createService,
    deleteService,
    rotateSecret,
    updatePermissions,
    copyToClipboard
  };
};
