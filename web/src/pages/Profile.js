import React, { lazy, useState } from 'react';
import { useAuth } from '../hooks/useAuth';
import { useNavigation } from '../hooks/useNavigation';
import { apiClient } from '../utils/apiClient';
import {
  User,
  Shield,
  Calendar,
  Lock,
  Save,
  X,
  Edit,
  ArrowLeft,
  Eye,
  EyeOff
} from 'lucide-react';
import { authService } from '../services/authService';

const Profile = ({ onBack }) => {
  const { user, isAdmin, refresh } = useAuth();

  const [editing, setEditing] = useState(false);
  const [changingPassword, setChangingPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [showPasswords, setShowPasswords] = useState({
    current: false,
    new: false,
    confirm: false
  });

  // profile data form
  const [profileData, setProfileData] = useState({
    username: user?.username || '',
  });

  // password change form
  const [passwordData, setPasswordData] = useState({
    currentPassword: '',
    newPassword: '',
    confirmPassword: ''
  });

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

  // profile editing
  const handleProfileChange = (field) => (e) => {
    setProfileData(prev => ({ ...prev, [field]: e.target.value }));
    clearMessages();
  };

  const handlePasswordChange = (field) => (e) => {
    setPasswordData(prev => ({ ...prev, [field]: e.target.value }));
    clearMessages();
  };

  const togglePasswordVisibility = (field) => {
    setShowPasswords(prev => ({ ...prev, [field]: !prev[field] }));
  };

  const handleSaveProfile = async (e) => {
    e.preventDefault();
    setLoading(true);
    clearMessages();

    try {
      await authService.updateUser(user.id, {
        username: profileData.username,
      });

      showMessage('Profile updated successfully!');
      setEditing(false);
      refresh();
    } catch (err) {
      showMessage(err.message || 'Failed to update profile', true);
    } finally {
      setLoading(false);
      refresh();
    }
  };

  const handleChangePassword = async (e) => {
    e.preventDefault();

    // surface ctrl
    if (passwordData.newPassword !== passwordData.confirmPassword) {
      showMessage('New passwords do not match', true);
      return;
    }

    if (passwordData.newPassword.length < 6) {
      showMessage('New password must be at least 6 characters', true);
      return;
    }

    setLoading(true);
    clearMessages();

    try {
      await apiClient.put('/auth/change-password', {
        currentPassword: passwordData.currentPassword,
        newPassword: passwordData.newPassword,
      });

      showMessage('Password changed successfully!');
      setChangingPassword(false);
      setPasswordData({ currentPassword: '', newPassword: '', confirmPassword: '' });
    } catch (err) {
      showMessage(err.message || 'Failed to change password', true);
    } finally {
      setLoading(false);
    }
  };

  // cancel
  const handleCancelEdit = () => {
    setProfileData({
      username: user?.username || '',
    });
    setEditing(false);
    clearMessages();
  };

  const handleCancelPasswordChange = () => {
    setPasswordData({ currentPassword: '', newPassword: '', confirmPassword: '' });
    setChangingPassword(false);
    clearMessages();
  };

  const renderPasswordInput = (field, placeholder, value, onChange) => {
    const isVisible = showPasswords[field];
    return (
      <div className="relative">
        <input
          type={isVisible ? 'text' : 'password'}
          placeholder={placeholder}
          value={value}
          onChange={onChange}
          className="w-full px-3 py-2 pr-10 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          required
        />
        <button
          type="button"
          onClick={() => togglePasswordVisibility(field)}
          className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-600"
        >
          {isVisible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
      </div>
    );
  };

  return (
    <div className="max-w-4xl mx-auto">
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
        </div>

        <h1 className="text-3xl font-bold text-gray-900 mt-4">Profile Settings</h1>
        <p className="text-gray-600 mt-2">Manage your account information and security settings</p>
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

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">

        {/* ===== PROFILE INFORMATION ===== */}
        <div className="lg:col-span-2">
          <div className="bg-white shadow rounded-lg p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-medium text-gray-900">Profile Information</h2>
              {!editing && (
                <button
                  onClick={() => setEditing(true)}
                  className="flex items-center px-3 py-2 text-sm text-blue-600 hover:text-blue-700 transition-colors"
                >
                  <Edit className="h-4 w-4 mr-1" />
                  Edit
                </button>
              )}
            </div>

            {editing ? (
              <form onSubmit={handleSaveProfile} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Username
                  </label>
                  <input
                    type="text"
                    value={profileData.username}
                    onChange={handleProfileChange('username')}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                    required
                  />
                </div>

                <div className="flex space-x-3 pt-4">
                  <button
                    type="submit"
                    disabled={loading}
                    className="flex items-center px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    <Save className="h-4 w-4 mr-2" />
                    {loading ? 'Saving...' : 'Save Changes'}
                  </button>
                  <button
                    type="button"
                    onClick={handleCancelEdit}
                    className="flex items-center px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-gray-500 transition-colors"
                  >
                    <X className="h-4 w-4 mr-2" />
                    Cancel
                  </button>
                </div>
              </form>
            ) : (
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700">Username</label>
                  <p className="mt-1 text-sm text-gray-900">{user?.username}</p>
                </div>
              </div>
            )}
          </div>

          {/* ===== CHANGE PASSWORD ===== */}
          <div className="bg-white shadow rounded-lg p-6 mt-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-medium text-gray-900">Security</h2>
              {!changingPassword && (
                <button
                  onClick={() => setChangingPassword(true)}
                  className="flex items-center px-3 py-2 text-sm text-blue-600 hover:text-blue-700 transition-colors"
                >
                  <Lock className="h-4 w-4 mr-1" />
                  Change Password
                </button>
              )}
            </div>

            {changingPassword ? (
              <form onSubmit={handleChangePassword} className="space-y-4">
                {[
                  {
                    label: "Current Password",
                    renderedInput: renderPasswordInput(
                      'current',
                      'Enter current password',
                      passwordData.currentPassword,
                      handlePasswordChange('currentPassword')
                    ),
                  },
                  {
                    label: "New Password",
                    renderedInput: renderPasswordInput(
                      'new',
                      'Enter new password',
                      passwordData.newPassword,
                      handlePasswordChange('newPassword')
                    ),
                    message: "Minimum 6 characters",
                  },
                  {
                    label: "Confirm New Password",
                    renderedInput: renderPasswordInput(
                      'confirm',
                      'Confirm new password',
                      passwordData.confirmPassword,
                      handlePasswordChange('confirmPassword')
                    ),
                  },
                ].map(formElement => (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      {formElement.label}
                    </label>
                    {formElement.renderedInput}
                    {formElement.message && (<p>{formElement.message}</p>)}
                  </div>
                ))}

                <div className="flex space-x-3 pt-4">
                  <button
                    type="submit"
                    disabled={loading}
                    className="flex items-center px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    <Lock className="h-4 w-4 mr-2" />
                    {loading ? 'Changing...' : 'Change Password'}
                  </button>
                  <button
                    type="button"
                    onClick={handleCancelPasswordChange}
                    className="flex items-center px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-gray-500 transition-colors"
                  >
                    <X className="h-4 w-4 mr-2" />
                    Cancel
                  </button>
                </div>
              </form>
            ) : (
              <div>
                <p className="text-sm text-gray-600">
                  Choose a strong password to keep your account secure.
                </p>
              </div>
            )}
          </div>
        </div>

        {/* ===== ACCOUNT INFO SIDEBAR ===== */}
        <div className="space-y-6">

          {/* Account Overview */}
          <div className="bg-white shadow rounded-lg p-6">
            <h3 className="text-lg font-medium text-gray-900 mb-4">Account Overview</h3>

            <div className="space-y-4">
              <div className="flex items-center">
                <User className="h-5 w-5 text-gray-400 mr-3" />
                <div>
                  <p className="text-sm font-medium text-gray-700">Role</p>
                  <div className="flex items-center space-x-2">
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${isAdmin()
                      ? 'bg-yellow-100 text-yellow-800'
                      : 'bg-blue-100 text-blue-800'
                      }`}>
                      {user?.role || 'user'}
                    </span>
                    {isAdmin() && <Shield className="h-4 w-4 text-yellow-500" />}
                  </div>
                </div>
              </div>

              <div className="flex items-center">
                <Calendar className="h-5 w-5 text-gray-400 mr-3" />
                <div>
                  <p className="text-sm font-medium text-gray-700">User since</p>
                  <p className="text-sm text-gray-500">
                    {user?.createdAt
                      ? new Date(user.createdAt).toLocaleDateString()
                      : 'Unknown'
                    }
                  </p>
                </div>
              </div>

              {user?.lastLogin && (
                <div className="flex items-center">
                  <Lock className="h-5 w-5 text-gray-400 mr-3" />
                  <div>
                    <p className="text-sm font-medium text-gray-700">Last login</p>
                    <p className="text-sm text-gray-500">
                      {new Date(user.lastLogin).toLocaleDateString()}
                    </p>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Permissions */}
          {isAdmin() && (
            <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
              <div className="flex items-center">
                <Shield className="h-5 w-5 text-yellow-600 mr-2" />
                <h4 className="text-sm font-medium text-yellow-800">Administrator</h4>
              </div>
              <p className="mt-2 text-sm text-yellow-700">
                You have full access to all system features including user management and service accounts.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Profile;
