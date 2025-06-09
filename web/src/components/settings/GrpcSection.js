import React from 'react';

const GrpcSection = ({ config, updateConfig }) => (
  <div>
    <h2 className="text-lg font-medium text-gray-900 mb-4">gRPC Settings</h2>
    <div className="space-y-4">
      <div className="flex items-center">
        <input
          type="checkbox"
          checked={config.GRPC?.Enabled}
          onChange={(e) => updateConfig('GRPC', 'Enabled', e.target.checked)}
          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900">Enable gRPC Server</label>
      </div>
      
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700">Address</label>
          <input
            type="text"
            value={config.GRPC?.Address}
            onChange={(e) => updateConfig('GRPC', 'Address', e.target.value)}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700">Port</label>
          <input
            type="number"
            value={config.GRPC?.Port}
            onChange={(e) => updateConfig('GRPC', 'Port', parseInt(e.target.value))}
            className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
      </div>
    </div>
  </div>
);

export default GrpcSection;
