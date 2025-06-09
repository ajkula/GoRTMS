import React from 'react';

const StorageSection = ({ config, updateConfig }) => (
  <div>
    <h2 className="text-lg font-medium text-gray-900 mb-4">Storage Settings</h2>
    <div className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700">Storage Engine</label>
        <select
          value={config.Storage?.Engine}
          onChange={(e) => updateConfig('Storage', 'Engine', e.target.value)}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
        >
          <option value="memory">Memory</option>
          <option value="file">File (not yet implemented)</option>
        </select>
      </div>
      
      <div>
        <label className="block text-sm font-medium text-gray-700">Storage Path</label>
        <input
          type="text"
          value={config.Storage?.Path}
          onChange={(e) => updateConfig('Storage', 'Path', e.target.value)}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
        />
      </div>
      
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700">Retention (days)</label>
          <input
            type="number"
            value={parseInt(config.Storage?.RetentionDays)}
            onChange={(e) => updateConfig('Storage', 'RetentionDays', parseInt(e.target.value))}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700">Max Size (MB)</label>
          <input
            type="number"
            value={parseInt(config.Storage?.MaxSizeMB)}
            onChange={(e) => updateConfig('Storage', 'MaxSizeMB', parseInt(e.target.value))}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
      </div>
      
      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.Storage?.Sync}
          onChange={(e) => updateConfig('Storage', 'Sync', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Synchronous Writes</label>
      </div>
    </div>
  </div>
);

export default StorageSection;
