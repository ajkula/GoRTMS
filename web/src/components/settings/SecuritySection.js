import React from 'react';

const SecuritySection = ({ config, updateConfig }) => (
  <div>
    {console.log({ config })}
    <h2 className="text-lg font-medium text-gray-900 mb-4">Security Settings</h2>
    <div className="space-y-4">
      <h3 className="text-md font-medium text-gray-900 mb-2">JWT Auth Settings</h3>
      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.security?.EnableAuthentication}
          onChange={(e) => updateConfig('security', 'EnableAuthentication', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Enable Authentication</label>
      </div>

      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.security?.EnableAuthorization}
          onChange={(e) => updateConfig('security', 'EnableAuthorization', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Enable Authorization</label>
      </div>

      <div className="border-t pt-4">
        <div className="space-y-4">
          <h3 className="text-md font-medium text-gray-900 mb-2">HMAC Services Auth Settings</h3>
          <div className="flex items-center">
            <input
              type="checkbox"
              checked={config.security?.HMAC?.RequireTLS}
              onChange={(e) => updateConfig('security', 'HMAC.RequireTLS', e.target.checked)}
              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
            />
            <label className="ml-2 block text-sm text-gray-900">TLS Required</label>
          </div>

          <div className="flex items-center">
            <input
              type="checkbox"
              checked={config.security?.HMAC?.TimestampWindow}
              onChange={(e) => updateConfig('security', 'HMAC.TimestampWindow', e.target.checked)}
              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
            />
            <label className="ml-2 block text-sm text-gray-900">Timestamp Window</label>
          </div>
        </div>
      </div>
    </div>
  </div>
);

export default SecuritySection;
