import React from 'react';

const GeneralSection = ({ config, updateConfig }) => (
  <div>
    <h2 className="text-lg font-medium text-gray-900 mb-4">General Settings</h2>
    <div className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700">Node ID</label>
        <input
          type="text"
          value={config.General?.NodeID}
          onChange={(e) => updateConfig('General', 'NodeID', e.target.value)}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">Data Directory</label>
        <input
          type="text"
          value={config.General?.DataDir}
          onChange={(e) => updateConfig('General', 'DataDir', e.target.value)}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">Log Level</label>
        <select
          value={config.General?.LogLevel}
          onChange={(e) => updateConfig('General', 'LogLevel', e.target.value)}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
        >
          <option value="debug">Debug</option>
          <option value="info">Info</option>
          <option value="warn">Warning</option>
          <option value="error">Error</option>
        </select>
      </div>
      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.General?.Development}
          onChange={(e) => updateConfig('General', 'Development', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Development Mode</label>
      </div>
    </div>
  </div>
);

export default GeneralSection;
