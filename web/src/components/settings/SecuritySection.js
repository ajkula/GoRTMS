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
        <div>
          <label className="block text-sm font-medium text-gray-700">Admin Username</label>
          <input
            type="text"
            value={config.Security?.AdminUsername}
            onChange={(e) => updateConfig('Security', 'AdminUsername', e.target.value)}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700">Admin Password</label>
          <input
            type="password"
            value={config.Security?.AdminPassword}
            onChange={(e) => updateConfig('Security', 'AdminPassword', e.target.value)}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
      </div>
    </div>
  </div>
);

export default SecuritySection;
