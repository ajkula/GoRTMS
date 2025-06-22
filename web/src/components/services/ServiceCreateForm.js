import React, { useState } from 'react';
import { Save, X } from 'lucide-react';
import { useServicePermissions } from '../../hooks/useServicePermissions';
import PermissionBuilder from './PermissionBuilder';
import IPWhitelistManager from './IPWhitelistManager';

const ServiceCreateForm = ({ 
  domains,
  onSubmit, 
  onCancel, 
  loading
}) => {
  const [name, setName] = useState('');
  
  const {
    permissions,
    ipWhitelist,
    permissionBuilder,
    setPermissionBuilder,
    ipInput,
    setIpInput,
    addPermission,
    removePermission,
    addIP,
    removeIP
  } = useServicePermissions();

  const handleSubmit = (e) => {
    e.preventDefault();
    
    if (!name.trim()) {
      alert('Service name is required');
      return;
    }

    if (permissions.length === 0) {
      alert('At least one permission is required');
      return;
    }

    onSubmit({
      name: name.trim(),
      permissions,
      ipWhitelist
    });

    // Reset form
    setName('');
  };

  return (
    <div className="bg-white shadow rounded-lg p-6 mb-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-lg font-medium text-gray-900">Create New Service Account</h2>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <div>
          <label htmlFor="service-name" className="block text-sm font-medium text-gray-700 mb-1">
            Service Name *
          </label>
          <input
            id="service-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="Enter service name (e.g., my-app-prod)"
            required
          />
        </div>

        <PermissionBuilder
          domains={domains}
          permissions={permissions}
          permissionBuilder={permissionBuilder}
          setPermissionBuilder={setPermissionBuilder}
          onAdd={addPermission}
          onRemove={removePermission}
        />

        <IPWhitelistManager
          ipWhitelist={ipWhitelist}
          ipInput={ipInput}
          setIpInput={setIpInput}
          onAdd={addIP}
          onRemove={removeIP}
        />

        <div className="flex space-x-3 pt-4">
          <button
            type="submit"
            disabled={loading}
            className="flex items-center px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <Save className="h-4 w-4 mr-2" />
            {loading ? 'Creating...' : 'Create Service'}
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="flex items-center px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-gray-500 transition-colors"
          >
            <X className="h-4 w-4 mr-2" />
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
};

export default ServiceCreateForm;
