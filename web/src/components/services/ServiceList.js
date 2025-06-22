import React from 'react';
import { Shield, Clock, Edit, RotateCcw, Trash2, Save, X, CheckCircle, XCircle, AlertCircle } from 'lucide-react';

const ServiceList = ({ 
  services, 
  loading, 
  editingService,
  onEdit,
  onSave,
  onCancel,
  onRotateSecret,
  onDelete,
  formatDate
}) => {
  const getStatusBadge = (enabled, lastUsed) => {
    const neverUsed = !lastUsed || lastUsed === '0001-01-01T00:00:00Z';
    
    if (!enabled) {
      return (
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
          <XCircle className="h-3 w-3 mr-1" />
          Disabled
        </span>
      );
    }

    if (neverUsed) {
      return (
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
          <AlertCircle className="h-3 w-3 mr-1" />
          Not Used
        </span>
      );
    }

    return (
      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
        <CheckCircle className="h-3 w-3 mr-1" />
        Active
      </span>
    );
  };

  if (loading) {
    return (
      <div className="bg-white shadow rounded-lg">
        <div className="p-6 text-center text-gray-500">
          Loading services...
        </div>
      </div>
    );
  }

  if (services.length === 0) {
    return (
      <div className="bg-white shadow rounded-lg">
        <div className="p-6 text-center text-gray-500">
          <Shield className="h-12 w-12 mx-auto mb-4 text-gray-300" />
          <p>No service accounts yet</p>
          <p className="text-sm">Create your first service to get started</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white shadow rounded-lg">
      <div className="px-6 py-4 border-b border-gray-200">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-medium text-gray-900">Service Accounts</h2>
          <div className="flex items-center text-sm text-gray-500">
            <Shield className="h-4 w-4 mr-1" />
            {services.length} total services
          </div>
        </div>
      </div>

      <div className="overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Service
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Permissions
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Last Used
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {services.map((service) => (
              <tr key={service.id} className="hover:bg-gray-50">
                <td className="px-6 py-4 whitespace-nowrap">
                  <div className="flex items-center">
                    <div className="h-8 w-8 rounded-full bg-indigo-200 flex items-center justify-center mr-3">
                      <Shield className="h-4 w-4 text-indigo-600" />
                    </div>
                    <div>
                      <div className="text-sm font-medium text-gray-900">{service.name}</div>
                      <div className="text-sm text-gray-500">ID: {service.id}</div>
                    </div>
                  </div>
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  {getStatusBadge(service.enabled, service.lastUsed)}
                </td>
                <td className="px-6 py-4">
                  <div className="space-y-1">
                    {service.permissions.slice(0, 2).map((perm, index) => (
                      <span key={index} className="inline-block bg-blue-100 text-blue-800 text-xs px-2 py-1 rounded-full mr-1">
                        {perm}
                      </span>
                    ))}
                    {service.permissions.length > 2 && (
                      <span className="text-xs text-gray-500">+{service.permissions.length - 2} more</span>
                    )}
                  </div>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  <div className="flex items-center">
                    <Clock className="h-4 w-4 mr-1" />
                    {formatDate(service.lastUsed)}
                  </div>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                  <div className="flex space-x-2">
                    {editingService === service.id ? (
                      <>
                        <button
                          onClick={onSave}
                          className="text-green-600 hover:text-green-900 transition-colors"
                          title="Save changes"
                        >
                          <Save className="h-4 w-4" />
                        </button>
                        <button
                          onClick={onCancel}
                          className="text-gray-600 hover:text-gray-900 transition-colors"
                          title="Cancel"
                        >
                          <X className="h-4 w-4" />
                        </button>
                      </>
                    ) : (
                      <>
                        <button
                          onClick={() => onEdit(service)}
                          className="text-blue-600 hover:text-blue-900 transition-colors"
                          title="Edit permissions"
                        >
                          <Edit className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => onRotateSecret(service.id)}
                          className="text-orange-600 hover:text-orange-900 transition-colors"
                          title="Rotate secret"
                        >
                          <RotateCcw className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => onDelete(service.id, service.name)}
                          className="text-red-600 hover:text-red-900 transition-colors"
                          title="Delete service"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default ServiceList;
