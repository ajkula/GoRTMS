import React from 'react';

const SecuritySection = ({ config, updateConfig }) => (
  <div>
    <h2 className="text-lg font-medium text-gray-900 mb-4">Security Settings</h2>
    <div className="space-y-4">
      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.Security?.EnableAuthentication}
          onChange={(e) => updateConfig('Security', 'EnableAuthentication', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Enable Authentication</label>
      </div>

      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.Security?.EnableAuthorization}
          onChange={(e) => updateConfig('Security', 'EnableAuthorization', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Enable Authorization</label>
      </div>

      <div className="grid grid-cols-2 gap-4">
      </div>
    </div>
  </div>
);

export default SecuritySection;
