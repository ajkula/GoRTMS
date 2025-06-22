import React from 'react';
import { X } from 'lucide-react';

const PermissionBuilder = ({ 
  domains,
  permissions,
  permissionBuilder,
  setPermissionBuilder,
  onAdd,
  onRemove
}) => {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-3">
        Permissions *
      </label>
      
      <div className="border rounded-md p-4 bg-gray-50">
        <h4 className="text-sm font-medium text-gray-700 mb-3">Add Permission</h4>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Action</label>
            <select
              value={permissionBuilder.action}
              onChange={(e) => setPermissionBuilder(prev => ({ ...prev, action: e.target.value }))}
              className="w-full px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="*">All Actions (*)</option>
              <option value="publish">Publish</option>
              <option value="consume">Consume</option>
              <option value="manage">Manage</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Domain</label>
            <select
              value={permissionBuilder.domain}
              onChange={(e) => setPermissionBuilder(prev => ({ ...prev, domain: e.target.value }))}
              className="w-full px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              disabled={permissionBuilder.action === '*'}
            >
              <option value="*">All Domains (*)</option>
              {domains.map(domain => (
                <option key={domain.name} value={domain.name}>{domain.name}</option>
              ))}
            </select>
          </div>
          <div className="flex items-end">
            <button
              type="button"
              onClick={onAdd}
              className="w-full px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
            >
              Add
            </button>
          </div>
        </div>
      </div>
      
      {permissions.length > 0 && (
        <div className="mt-3">
          <h5 className="text-sm font-medium text-gray-700 mb-2">Current Permissions:</h5>
          <div className="space-y-2">
            {permissions.map((permission, index) => (
              <div key={index} className="flex items-center justify-between bg-blue-50 px-3 py-2 rounded-md">
                <span className="text-sm font-mono text-blue-800">{permission}</span>
                <button
                  type="button"
                  onClick={() => onRemove(permission)}
                  className="text-red-600 hover:text-red-800 transition-colors"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default PermissionBuilder;
