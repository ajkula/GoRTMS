// components/Settings.js (version avec imports propres)
import React, { useState } from 'react';
import { Save, RotateCcw, AlertTriangle, Loader } from 'lucide-react';
import { useSettings } from '../hooks/useSettings';
import {
  StatusMessages,
  SettingsNavigation,
  GeneralSection,
  HttpSection,
  SecuritySection,
  StorageSection,
  GrpcSection,
} from './settings';

const Settings = () => {
  const [activeSection, setActiveSection] = useState('general');
  
  const {
    config,
    loading,
    saving,
    error,
    success,
    hasChanges,
    saveSettings,
    resetSettings,
    updateConfig,
    updateArrayField,
    addArrayItem,
    removeArrayItem,
    clearError,
    clearSuccess,
  } = useSettings();

  const handleResetSettings = async () => {
    if (!window.confirm('Are you sure you want to reset all settings to default values? This action cannot be undone.')) {
      return;
    }
    await resetSettings();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader className="h-6 w-6 animate-spin text-indigo-600" />
        <span className="ml-2">Loading settings...</span>
      </div>
    );
  }

  if (!config) {
    return (
      <div className="text-center">
        <AlertTriangle className="h-8 w-8 text-red-500 mx-auto mb-2" />
        <h3 className="text-lg font-medium text-red-800">Failed to load settings</h3>
        <p className="text-sm text-red-600 mt-1">{error}</p>
      </div>
    );
  }

  const sectionComponents = {
    general: GeneralSection,
    http: HttpSection,
    security: SecuritySection,
    storage: StorageSection,
    grpc: GrpcSection,
  };

  const ActiveSectionComponent = sectionComponents[activeSection] || GeneralSection;

  return (
    <div className="max-w-7xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">System Settings</h1>
        <p className="text-gray-600 mt-1">Configure your GoRTMS instance</p>
      </div>

      <StatusMessages 
        error={error} 
        success={success} 
        onClearError={clearError} 
        onClearSuccess={clearSuccess} 
      />

      <div className="flex gap-6">
        <SettingsNavigation 
          activeSection={activeSection} 
          onSectionChange={setActiveSection} 
        />

        <div className="flex-1 bg-white rounded-lg shadow-sm">
          <div className="p-6">
            <ActiveSectionComponent
              config={config}
              updateConfig={updateConfig}
              updateArrayField={updateArrayField}
              addArrayItem={addArrayItem}
              removeArrayItem={removeArrayItem}
            />
          </div>

          <div className="bg-gray-50 px-6 py-3 flex justify-between items-center rounded-b-lg">
            <button
              onClick={handleResetSettings}
              disabled={saving}
              className="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
            >
              <RotateCcw className="h-4 w-4 mr-2" />
              Reset to Defaults
            </button>

            <div className="flex items-center space-x-3">
              {hasChanges && (
                <span className="text-sm text-amber-600">You have unsaved changes</span>
              )}
              <button
                onClick={saveSettings}
                disabled={saving || !hasChanges}
                className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
              >
                {saving ? (
                  <Loader className="h-4 w-4 mr-2 animate-spin" />
                ) : (
                  <Save className="h-4 w-4 mr-2" />
                )}
                {saving ? 'Saving...' : 'Save Settings'}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Settings;
