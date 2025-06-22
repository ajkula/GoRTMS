import React from 'react';
import PermissionBuilder from './PermissionBuilder';
import IPWhitelistManager from './IPWhitelistManager';

const ServiceEditForm = ({ 
  service,
  domains,
  editData,
  setEditData,
  useServicePermissions
}) => {
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
  } = useServicePermissions(editData.permissions, editData.ipWhitelist);

  // update parent state when permissions change
  React.useEffect(() => {
    setEditData(prev => ({
      ...prev,
      permissions,
      ipWhitelist
    }));
  }, [permissions, ipWhitelist, setEditData]);

  return (
    <div className="border-t border-gray-200 p-6 bg-gray-50">
      <h3 className="text-lg font-medium text-gray-900 mb-4">
        Edit Service: {service.name}
      </h3>
      
      <div className="space-y-6">
        <div className="flex items-center">
          <input
            type="checkbox"
            checked={editData.enabled}
            onChange={(e) => setEditData(prev => ({ ...prev, enabled: e.target.checked }))}
            className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
          />
          <label className="ml-2 text-sm text-gray-700">
            Service Enabled
          </label>
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
      </div>
    </div>
  );
};

export default ServiceEditForm;
