import React from 'react';

const HttpSection = ({ config, updateConfig, updateArrayField, addArrayItem, removeArrayItem }) => (
  <div>
    <h2 className="text-lg font-medium text-gray-900 mb-4">HTTP Server Settings</h2>
    <div className="space-y-4">
      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.HTTP?.Enabled}
          onChange={(e) => updateConfig('HTTP', 'Enabled', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Enable HTTP Server</label>
      </div>
      
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700">Address</label>
          <input
            type="text"
            value={config.HTTP?.Address}
            onChange={(e) => updateConfig('HTTP', 'Address', e.target.value)}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700">Port</label>
          <input
            type="number"
            value={config.HTTP?.Port}
            onChange={(e) => updateConfig('HTTP', 'Port', parseInt(e.target.value))}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
      </div>
      
      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.HTTP?.TLS ?? false}
          onChange={(e) => updateConfig('HTTP', 'TLS', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Enable TLS</label>
      </div>

      {config.HTTP?.TLS && (
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Certificate File</label>
            <input
              type="text"
              value={config.HTTP?.CertFile}
              onChange={(e) => updateConfig('HTTP', 'CertFile', e.target.value)}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Key File</label>
            <input
              type="text"
              value={config.HTTP?.KeyFile}
              onChange={(e) => updateConfig('HTTP', 'KeyFile', e.target.value)}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
            />
          </div>
        </div>
      )}

      <div className="border-t pt-4">
        <h3 className="text-md font-medium text-gray-900 mb-2">CORS Settings</h3>
        <div className="flex items-center mb-2">
          <input
            type="checkbox"
            checked={config.HTTP?.CORS?.Enabled}
            onChange={(e) => updateConfig('HTTP', 'CORS.Enabled', e.target.checked)}
            className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
          />
          <label className="ml-2 block text-sm text-gray-900">Enable CORS</label>
        </div>
        
        {config.HTTP?.CORS?.Enabled && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Allowed Origins</label>
            {config.HTTP?.CORS?.AllowedOrigins?.map((origin, index) => (
              <div key={index} className="flex items-center mb-2">
                <input
                  type="text"
                  value={origin}
                  onChange={(e) => updateArrayField('HTTP', 'CORS.AllowedOrigins', index, e.target.value)}
                  className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                />
                <button
                  onClick={() => removeArrayItem('HTTP', 'CORS.AllowedOrigins', index)}
                  className="ml-2 px-3 py-1 bg-red-100 text-red-700 rounded-md hover:bg-red-200"
                >
                  Remove
                </button>
              </div>
            ))}
            <button
              onClick={() => addArrayItem('HTTP', 'CORS.AllowedOrigins')}
              className="px-3 py-1 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200"
            >
              Add Origin
            </button>
          </div>
        )}
      </div>

      <div className="border-t pt-4">
        <h3 className="text-md font-medium text-gray-900 mb-2">JWT Settings</h3>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Expiration (minutes)</label>
            <input
              type="number"
              value={config.HTTP?.JWT?.ExpirationMinutes}
              onChange={(e) => updateConfig('HTTP', 'JWT.ExpirationMinutes', parseInt(e.target.value))}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
);

export default HttpSection;
