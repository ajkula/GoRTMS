import React, { useState, useEffect } from 'react';
import { useAuth } from '../hooks/useAuth';
import { useServices } from '../hooks/useServices';
import { useServicePermissions } from '../hooks/useServicePermissions';
import { ArrowLeft, Plus, Copy } from 'lucide-react';
import api from '../api';
import ServiceList from '../components/services/ServiceList';
import ServiceCreateForm from '../components/services/ServiceCreateForm';
import ServiceEditForm from '../components/services/ServiceEditForm';

const ServiceManagement = ({ onBack }) => {
  const { isAuthenticated } = useAuth();
  const {
    services,
    loading,
    message,
    error,
    clearMessages,
    createService,
    deleteService,
    rotateSecret,
    updatePermissions,
    copyToClipboard
  } = useServices();

  const [domains, setDomains] = useState([]);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [editingService, setEditingService] = useState(null);
  const [editData, setEditData] = useState({
    permissions: [],
    ipWhitelist: [],
    enabled: true
  });

  const loadDomains = async () => {
    try {
      const domainsList = await api.getDomains();
      setDomains(domainsList || []);
    } catch (err) {
      console.error('failed to load domains:', err);
    }
  };

  useEffect(() => {
    if (isAuthenticated) {
      loadDomains();
    }
  }, [isAuthenticated]);

  const handleCreateService = async (serviceData) => {
    try {
      await createService(serviceData);
      setShowCreateForm(false);
    } catch (err) {
      // noop
    }
  };

  const handleEditService = (service) => {
    setEditingService(service.id);
    setEditData({
      permissions: [...service.permissions],
      ipWhitelist: [...service.ipWhitelist],
      enabled: service.enabled
    });
    clearMessages();
  };

  const handleSavePermissions = async () => {
    try {
      await updatePermissions(editingService, editData);
      setEditingService(null);
    } catch (err) {
      // noop
    }
  };

  const handleCancelEdit = () => {
    setEditingService(null);
    clearMessages();
  };

  const handleRotateSecret = async (serviceId) => {
    if (!confirm('This will invalidate the current secret. Continue?')) return;
    try {
      await rotateSecret(serviceId);
    } catch (err) {
      // noop
    }
  };

  const handleDeleteService = async (serviceId, serviceName) => {
    if (!confirm(`Delete service "${serviceName}"? This cannot be undone.`)) return;
    try {
      await deleteService(serviceId, serviceName);
    } catch (err) {
      // noop
    }
  };

  const formatDate = (dateString) => {
    if (!dateString || dateString === '0001-01-01T00:00:00Z') return 'Never';
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  if (!isAuthenticated) {
    return (
      <div className="max-w-4xl mx-auto">
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          <p>Authentication required to access service management.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-4">
            <button
              onClick={onBack}
              className="flex items-center text-gray-600 hover:text-gray-900 transition-colors"
            >
              <ArrowLeft className="h-5 w-5 mr-1" />
              Back to Dashboard
            </button>
          </div>
          
          {!showCreateForm && (
            <button
              onClick={() => setShowCreateForm(true)}
              className="flex items-center px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
            >
              <Plus className="h-4 w-4 mr-2" />
              Create Service
            </button>
          )}
        </div>
        
        <h1 className="text-3xl font-bold text-gray-900 mt-4">Service Account Management</h1>
        <p className="text-gray-600 mt-2">Manage HMAC service accounts for API access</p>
      </div>

      {message && (
        <div className="mb-6 bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded-md">
          {message.includes('SECRET') ? (
            <div className="flex items-center justify-between">
              <span>{message}</span>
              <button
                onClick={() => copyToClipboard(message.split(': ')[1])}
                className="flex items-center text-green-800 hover:text-green-900 ml-4"
              >
                <Copy className="h-4 w-4 mr-1" />
                Copy
              </button>
            </div>
          ) : (
            message
          )}
        </div>
      )}
      
      {error && (
        <div className="mb-6 bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          {error}
        </div>
      )}

      {/* Create Service */}
      {showCreateForm && (
        <ServiceCreateForm
          domains={domains}
          onSubmit={handleCreateService}
          onCancel={() => setShowCreateForm(false)}
          loading={false}
        />
      )}

      <ServiceList
        services={services}
        loading={loading}
        editingService={editingService}
        onEdit={handleEditService}
        onSave={handleSavePermissions}
        onCancel={handleCancelEdit}
        onRotateSecret={handleRotateSecret}
        onDelete={handleDeleteService}
        formatDate={formatDate}
      />

      {editingService && (
        <ServiceEditForm
          service={services.find(s => s.id === editingService)}
          domains={domains}
          editData={editData}
          setEditData={setEditData}
        />
      )}
    </div>
  );
};

export default ServiceManagement;
