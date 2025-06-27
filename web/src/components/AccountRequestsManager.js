import React, { useState, useEffect } from 'react';
import { 
  UserCheck, 
  UserX, 
  Clock, 
  Calendar, 
  Shield, 
  User,
  CheckCircle,
  XCircle,
  Eye,
  Filter,
  RefreshCw
} from 'lucide-react';

const AccountRequestsManager = ({ apiClient }) => {
  const [requests, setRequests] = useState([]);
  const [loading, setLoading] = useState(true);
  const [processing, setProcessing] = useState({});
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [filter, setFilter] = useState('pending'); // 'all', 'pending', 'approved', 'rejected'

  const clearMessages = () => {
    setMessage('');
    setError('');
  };

  const showMessage = (msg, isError = false) => {
    if (isError) {
      setError(msg);
      setMessage('');
    } else {
      setMessage(msg);
      setError('');
    }
    setTimeout(clearMessages, 5000);
  };

  const loadRequests = async () => {
    try {
      setLoading(true);
      clearMessages();
      
      const params = filter !== 'all' ? `?status=${filter}` : '';
      const response = await apiClient.get(`/admin/account-requests${params}`);
      setRequests(response.requests || []);
    } catch (err) {
      showMessage(err.message || 'Failed to load account requests', true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadRequests();
  }, [filter]);

  const handleReview = async (requestId, approve, approvedRole = null, rejectReason = '') => {
    setProcessing(prev => ({ ...prev, [requestId]: true }));
    clearMessages();

    try {
      const payload = {
        approve,
        ...(approve && approvedRole && { approvedRole }),
        ...((!approve && rejectReason) && { rejectReason })
      };

      await apiClient.post(`/admin/account-requests/${requestId}/review`, payload);
      
      showMessage(
        approve 
          ? 'Account request approved successfully. User account has been created.'
          : 'Account request rejected successfully.'
      );
      
      loadRequests();
    } catch (err) {
      showMessage(err.message || 'Failed to review account request', true);
    } finally {
      setProcessing(prev => ({ ...prev, [requestId]: false }));
    }
  };

  const handleDelete = async (requestId) => {
    if (!window.confirm('Are you sure you want to delete this request?')) {
      return;
    }

    setProcessing(prev => ({ ...prev, [requestId]: true }));
    clearMessages();

    try {
      await apiClient.delete(`/admin/account-requests/${requestId}`);
      showMessage('Account request deleted successfully');
      loadRequests();
    } catch (err) {
      showMessage(err.message || 'Failed to delete account request', true);
    } finally {
      setProcessing(prev => ({ ...prev, [requestId]: false }));
    }
  };

  const formatDate = (dateString) => {
    if (!dateString) return 'N/A';
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const getStatusBadge = (status) => {
    const styles = {
      pending: 'bg-yellow-100 text-yellow-800',
      approved: 'bg-green-100 text-green-800',
      rejected: 'bg-red-100 text-red-800'
    };

    const icons = {
      pending: Clock,
      approved: CheckCircle,
      rejected: XCircle
    };

    const Icon = icons[status] || Clock;

    return (
      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${styles[status]}`}>
        <Icon className="h-3 w-3 mr-1" />
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </span>
    );
  };

  const getRoleBadge = (role) => {
    const isAdminRole = role === 'admin';
    return (
      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
        isAdminRole 
          ? 'bg-yellow-100 text-yellow-800' 
          : 'bg-blue-100 text-blue-800'
      }`}>
        {isAdminRole && <Shield className="h-3 w-3 mr-1" />}
        {role}
      </span>
    );
  };

  const pendingRequests = requests.filter(req => req.status === 'pending');

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-gray-900">Account Requests</h2>
          <p className="text-gray-600 mt-1">
            Review and manage user account requests
            {pendingRequests.length > 0 && (
              <span className="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
                {pendingRequests.length} pending
              </span>
            )}
          </p>
        </div>
        <button
          onClick={loadRequests}
          disabled={loading}
          className="flex items-center px-3 py-2 text-gray-600 hover:text-gray-900 transition-colors"
        >
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Messages */}
      {message && (
        <div className="bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded-md">
          {message}
        </div>
      )}
      
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          {error}
        </div>
      )}

      {/* Filter */}
      <div className="flex items-center space-x-4">
        <div className="flex items-center space-x-2">
          <Filter className="h-4 w-4 text-gray-500" />
          <span className="text-sm font-medium text-gray-700">Filter:</span>
        </div>
        <div className="flex space-x-2">
          {[
            { value: 'all', label: 'All' },
            { value: 'pending', label: 'Pending' },
            { value: 'approved', label: 'Approved' },
            { value: 'rejected', label: 'Rejected' }
          ].map(option => (
            <button
              key={option.value}
              onClick={() => setFilter(option.value)}
              className={`px-3 py-1 text-xs font-medium rounded-full transition-colors ${
                filter === option.value
                  ? 'bg-blue-100 text-blue-800'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
              }`}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      {/* Requests List */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        {loading ? (
          <div className="p-6 text-center text-gray-500">
            <div className="animate-spin h-6 w-6 border-2 border-blue-600 border-t-transparent rounded-full mx-auto mb-2"></div>
            Loading requests...
          </div>
        ) : requests.length === 0 ? (
          <div className="p-6 text-center text-gray-500">
            <UserCheck className="h-12 w-12 mx-auto mb-3 text-gray-300" />
            <p>No account requests found</p>
            {filter !== 'all' && (
              <button
                onClick={() => setFilter('all')}
                className="text-blue-600 hover:text-blue-500 text-sm mt-1"
              >
                Show all requests
              </button>
            )}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    User
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Requested Role
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Submitted
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {requests.map((request) => (
                  <tr key={request.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <div className="h-8 w-8 rounded-full bg-gray-200 flex items-center justify-center mr-3">
                          <User className="h-4 w-4 text-gray-600" />
                        </div>
                        <div>
                          <div className="text-sm font-medium text-gray-900">
                            {request.username}
                          </div>
                          <div className="text-sm text-gray-500">
                            ID: {request.id.slice(0, 8)}...
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {getRoleBadge(request.requestedRole)}
                      {request.approvedRole && request.approvedRole !== request.requestedRole && (
                        <div className="mt-1">
                          <span className="text-xs text-gray-500">Approved as: </span>
                          {getRoleBadge(request.approvedRole)}
                        </div>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div>
                        {getStatusBadge(request.status)}
                        {request.reviewedBy && (
                          <div className="text-xs text-gray-500 mt-1">
                            by {request.reviewedBy}
                          </div>
                        )}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      <div className="flex items-center">
                        <Calendar className="h-4 w-4 mr-1" />
                        {formatDate(request.createdAt)}
                      </div>
                      {request.reviewedAt && (
                        <div className="flex items-center mt-1 text-xs">
                          <Clock className="h-3 w-3 mr-1" />
                          Reviewed: {formatDate(request.reviewedAt)}
                        </div>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                      {request.status === 'pending' ? (
                        <div className="flex space-x-2">
                          <button
                            onClick={() => handleReview(request.id, true)}
                            disabled={processing[request.id]}
                            className="inline-flex items-center px-3 py-1 text-xs font-medium text-green-700 bg-green-100 rounded-md hover:bg-green-200 focus:outline-none focus:ring-2 focus:ring-green-500 disabled:opacity-50 transition-colors"
                          >
                            <UserCheck className="h-3 w-3 mr-1" />
                            Approve
                          </button>
                          <button
                            onClick={() => {
                              const reason = window.prompt('Rejection reason (optional):');
                              if (reason !== null) {
                                handleReview(request.id, false, null, reason);
                              }
                            }}
                            disabled={processing[request.id]}
                            className="inline-flex items-center px-3 py-1 text-xs font-medium text-red-700 bg-red-100 rounded-md hover:bg-red-200 focus:outline-none focus:ring-2 focus:ring-red-500 disabled:opacity-50 transition-colors"
                          >
                            <UserX className="h-3 w-3 mr-1" />
                            Reject
                          </button>
                        </div>
                      ) : (
                        <div className="flex space-x-2">
                          <button
                            onClick={() => handleDelete(request.id)}
                            disabled={processing[request.id]}
                            className="inline-flex items-center px-3 py-1 text-xs font-medium text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-gray-500 disabled:opacity-50 transition-colors"
                          >
                            <XCircle className="h-3 w-3 mr-1" />
                            Delete
                          </button>
                          {request.rejectReason && (
                            <button
                              onClick={() => alert(`Rejection reason: ${request.rejectReason}`)}
                              className="inline-flex items-center px-3 py-1 text-xs font-medium text-blue-700 bg-blue-100 rounded-md hover:bg-blue-200 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
                            >
                              <Eye className="h-3 w-3 mr-1" />
                              View Reason
                            </button>
                          )}
                        </div>
                      )}
                      {processing[request.id] && (
                        <div className="inline-flex items-center ml-2">
                          <div className="animate-spin h-3 w-3 border border-gray-400 border-t-transparent rounded-full"></div>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};

export default AccountRequestsManager;
