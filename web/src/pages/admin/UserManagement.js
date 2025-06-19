import React, { useState, useEffect } from 'react';
import { useAuth } from '../../hooks/useAuth';
import { apiClient } from '../../utils/apiClient';
import { 
  Users, 
  UserPlus, 
  Shield, 
  User,
  Calendar,
  Clock,
  Save,
  X,
  Edit,
  ArrowLeft,
  Eye,
  EyeOff,
  CheckCircle,
  XCircle
} from 'lucide-react';

const UserManagement = ({ onBack }) => {
  const { user: currentUser, isAdmin } = useAuth();
  
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [showPassword, setShowPassword] = useState(false);

  // Create user form data
  const [newUser, setNewUser] = useState({
    username: '',
    password: '',
    role: 'user'
  });

  // Access control - only admins can access this page
  if (!isAdmin()) {
    return (
      <div className="max-w-4xl mx-auto">
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          <p>Access denied. Administrator privileges required.</p>
        </div>
      </div>
    );
  }

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

  const loadUsers = async () => {
    try {
      setLoading(true);
      const response = await apiClient.get('/admin/users');
      setUsers(response);
    } catch (err) {
      showMessage(err.message || 'Failed to load users', true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadUsers();
  }, []);

  const handleInputChange = (field) => (e) => {
    setNewUser(prev => ({ ...prev, [field]: e.target.value }));
    clearMessages();
  };

  const handleCreateUser = async (e) => {
    e.preventDefault();
    
    if (!newUser.username.trim() || !newUser.password.trim()) {
      showMessage('Username and password are required', true);
      return;
    }

    if (newUser.password.length < 6) {
      showMessage('Password must be at least 6 characters', true);
      return;
    }

    setCreating(true);
    clearMessages();

    try {
      await apiClient.post('/admin/users', {
        username: newUser.username.trim(),
        password: newUser.password,
        role: newUser.role
      });
      
      showMessage(`User '${newUser.username}' created successfully!`);
      setNewUser({ username: '', password: '', role: 'user' });
      setShowCreateForm(false);
      loadUsers();
    } catch (err) {
      showMessage(err.message || 'Failed to create user', true);
    } finally {
      setCreating(false);
    }
  };

  const handleCancelCreate = () => {
    setNewUser({ username: '', password: '', role: 'user' });
    setShowCreateForm(false);
    clearMessages();
  };

  const formatDate = (dateString) => {
    if (!dateString) return 'Never';
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
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

  const getStatusBadge = (enabled) => {
    return (
      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
        enabled 
          ? 'bg-green-100 text-green-800' 
          : 'bg-red-100 text-red-800'
      }`}>
        {enabled ? (
          <>
            <CheckCircle className="h-3 w-3 mr-1" />
            Active
          </>
        ) : (
          <>
            <XCircle className="h-3 w-3 mr-1" />
            Disabled
          </>
        )}
      </span>
    );
  };

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
              <UserPlus className="h-4 w-4 mr-2" />
              Create User
            </button>
          )}
        </div>
        
        <h1 className="text-3xl font-bold text-gray-900 mt-4">User Management</h1>
        <p className="text-gray-600 mt-2">Manage system users and their permissions</p>
      </div>

      {/* Messages */}
      {message && (
        <div className="mb-6 bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded-md">
          {message}
        </div>
      )}
      
      {error && (
        <div className="mb-6 bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md">
          {error}
        </div>
      )}

      {/* Create User Form */}
      {showCreateForm && (
        <div className="bg-white shadow rounded-lg p-6 mb-6">
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-lg font-medium text-gray-900">Create New User</h2>
          </div>

          <form onSubmit={handleCreateUser} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Username *
                </label>
                <input
                  type="text"
                  value={newUser.username}
                  onChange={handleInputChange('username')}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  placeholder="Enter username"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Role
                </label>
                <select
                  value={newUser.role}
                  onChange={handleInputChange('role')}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                >
                  <option value="user">User</option>
                  <option value="admin">Administrator</option>
                </select>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Password *
              </label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={newUser.password}
                  onChange={handleInputChange('password')}
                  className="w-full px-3 py-2 pr-10 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  placeholder="Enter password (min 6 characters)"
                  required
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600"
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <p className="mt-1 text-xs text-gray-500">Minimum 6 characters</p>
            </div>

            <div className="flex space-x-3 pt-4">
              <button
                type="submit"
                disabled={creating}
                className="flex items-center px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <Save className="h-4 w-4 mr-2" />
                {creating ? 'Creating...' : 'Create User'}
              </button>
              <button
                type="button"
                onClick={handleCancelCreate}
                className="flex items-center px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-gray-500 transition-colors"
              >
                <X className="h-4 w-4 mr-2" />
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Users List */}
      <div className="bg-white shadow rounded-lg">
        <div className="px-6 py-4 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-medium text-gray-900">System Users</h2>
            <div className="flex items-center text-sm text-gray-500">
              <Users className="h-4 w-4 mr-1" />
              {users.length} total users
            </div>
          </div>
        </div>

        {loading ? (
          <div className="p-6 text-center text-gray-500">
            Loading users...
          </div>
        ) : users.length === 0 ? (
          <div className="p-6 text-center text-gray-500">
            No users found
          </div>
        ) : (
          <div className="overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    User
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Role
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Created
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Last Login
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {users.map((user) => (
                  <tr key={user.id} className={user.id === currentUser?.id ? 'bg-blue-50' : 'hover:bg-gray-50'}>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <div className="h-8 w-8 rounded-full bg-indigo-200 flex items-center justify-center mr-3">
                          <User className="h-4 w-4 text-indigo-600" />
                        </div>
                        <div>
                          <div className="text-sm font-medium text-gray-900">
                            {user.username}
                            {user.id === currentUser?.id && (
                              <span className="ml-2 text-xs text-blue-600 font-normal">(You)</span>
                            )}
                          </div>
                          <div className="text-sm text-gray-500">ID: {user.id.slice(0, 8)}...</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {getRoleBadge(user.role)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {getStatusBadge(user.enabled)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      <div className="flex items-center">
                        <Calendar className="h-4 w-4 mr-1" />
                        {formatDate(user.createdAt)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      <div className="flex items-center">
                        <Clock className="h-4 w-4 mr-1" />
                        {formatDate(user.lastLogin)}
                      </div>
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

export default UserManagement;
