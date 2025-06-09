import { useState, useEffect, useCallback, useRef } from 'react';
import api from '../api';

export const useSettings = () => {
  const [config, setConfig] = useState(null);
  const [originalConfig, setOriginalConfig] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [hasChanges, setHasChanges] = useState(false);

  // Use refs to avoid dependency issues
  const originalConfigRef = useRef(null);

  // Update refs when state changes
  useEffect(() => {
    originalConfigRef.current = originalConfig;
  }, [originalConfig]);

  const deepClone = obj => JSON.parse(JSON.stringify(obj));

  const loadSettings = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      
      const data = await api.getSettings();
      const newConfig = data.config;
      const originalCopy = deepClone(newConfig);
      
      setConfig(newConfig);
      setOriginalConfig(originalCopy);
      setHasChanges(false);
    } catch (err) {
      console.error('Error loading settings:', err);
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  const saveSettings = useCallback(async () => {
    if (!config) return;

    try {
      setSaving(true);
      setError(null);
      setSuccess(null);

      const response = await api.updateSettings(config);
      setSuccess(response.message);
      
      // Update original config to new saved state
      const newOriginal = deepClone(config);
      setOriginalConfig(newOriginal);
      setHasChanges(false);

      // Auto-hide after 3 seconds
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      console.error('Error saving settings:', err);
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }, [config]);

  const resetSettings = useCallback(async () => {
    try {
      setSaving(true);
      setError(null);
      setSuccess(null);

      const response = await api.resetSettings();
      const newConfig = response.config;
      const originalCopy = deepClone(newConfig);
      
      setConfig(newConfig);
      setOriginalConfig(originalCopy);
      setHasChanges(false);
      setSuccess('Settings reset to defaults successfully');

      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      console.error('Error resetting settings:', err);
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }, []);

  const updateConfig = useCallback((section, field, value) => {
    setConfig(prevConfig => {
      if (!prevConfig) return prevConfig;

      const newConfig = deepClone(prevConfig);
      
      if (!newConfig[section]) {
        newConfig[section] = {};
      }
      
      if (field.includes('.')) {
        // Nested field
        const [parentField, childField] = field.split('.');
        
        // Ensure parent field exists
        if (!newConfig[section][parentField]) {
          newConfig[section][parentField] = {};
        }
        
        newConfig[section][parentField][childField] = value;
      } else {
        newConfig[section][field] = value;
      }
      
      return newConfig;
    });
  }, []);

  const updateArrayField = useCallback((section, field, index, value) => {
    setConfig(prevConfig => {
      if (!prevConfig) return prevConfig;

      const newConfig = deepClone(prevConfig);
      
      if (field.includes('.')) {
        const [parentField, childField] = field.split('.');
        if (newConfig[section]?.[parentField]?.[childField]?.[index] !== undefined) {
          newConfig[section][parentField][childField][index] = value;
        }
      } else {
        if (newConfig[section]?.[field]?.[index] !== undefined) {
          newConfig[section][field][index] = value;
        }
      }
      
      return newConfig;
    });
  }, []);

  // Add item to array field
  const addArrayItem = useCallback((section, field, defaultValue = '') => {
    setConfig(prevConfig => {
      if (!prevConfig) return prevConfig;

      const newConfig = deepClone(prevConfig);
      
      if (field.includes('.')) {
        const [parentField, childField] = field.split('.');
        if (newConfig[section]?.[parentField]?.[childField]) {
          newConfig[section][parentField][childField].push(defaultValue);
        }
      } else {
        if (newConfig[section]?.[field]) {
          newConfig[section][field].push(defaultValue);
        }
      }
      
      return newConfig;
    });
  }, []);

  const removeArrayItem = useCallback((section, field, index) => {
    setConfig(prevConfig => {
      if (!prevConfig) return prevConfig;

      const newConfig = deepClone(prevConfig);
      
      if (field.includes('.')) {
        const [parentField, childField] = field.split('.');
        if (newConfig[section]?.[parentField]?.[childField]) {
          newConfig[section][parentField][childField].splice(index, 1);
        }
      } else {
        if (newConfig[section]?.[field]) {
          newConfig[section][field].splice(index, 1);
        }
      }
      
      return newConfig;
    });
  }, []);

  // Check for changes when config or originalConfig changes
  useEffect(() => {
    if (config && originalConfig) {
      const hasChanges = JSON.stringify(config) !== JSON.stringify(originalConfig);
      setHasChanges(hasChanges);
    }
  }, [config, originalConfig]);

  const clearError = useCallback(() => setError(null), []);

  const clearSuccess = useCallback(() => setSuccess(null), []);

  // Check if config has specific field
  const hasField = useCallback((section, field) => {
    if (!config || !config[section]) return false;
    
    if (field.includes('.')) {
      const [parentField, childField] = field.split('.');
      return config[section][parentField] && 
             config[section][parentField][childField] !== undefined;
    }
    
    return config[section][field] !== undefined;
  }, [config]);

  // Get field value safely
  const getFieldValue = useCallback((section, field, defaultValue = '') => {
    if (!config || !config[section]) return defaultValue;
    
    if (field.includes('.')) {
      const [parentField, childField] = field.split('.');
      return config[section][parentField]?.[childField] ?? defaultValue;
    }
    
    return config[section][field] ?? defaultValue;
  }, [config]);

  // Load settings on mount only
  useEffect(() => {
    loadSettings();
  }, []);

  return {
    // State
    config,
    loading,
    saving,
    error,
    success,
    hasChanges,
    
    // Actions
    loadSettings,
    saveSettings,
    resetSettings,
    updateConfig,
    updateArrayField,
    addArrayItem,
    removeArrayItem,
    clearError,
    clearSuccess,
    
    // Utilities
    hasField,
    getFieldValue,
  };
};