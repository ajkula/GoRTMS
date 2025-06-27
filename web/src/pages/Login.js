import React, { useState } from 'react';
import { useAuth } from '../hooks/useAuth';
import AccountRequestModal from '../components/AccountRequestModal';

const Login = ({ isClosing = false }) => {
  const [credentials, setCredentials] = useState({ username: '', password: '' });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showRequestModal, setShowRequestModal] = useState(false);

  const { login: logUserIn } = useAuth();

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      await logUserIn(credentials.username, credentials.password);
    } catch (err) {
      setError(err.message || 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const handleInputChange = (field) => (e) => {
    setCredentials(prev => ({ ...prev, [field]: e.target.value }));
    if (error) setError('');
  };

  return (
    <div className="h-screen flex overflow-hidden">
      {/* Partie gauche bleue avec animation */}
      <div
        className={`relative transition-all duration-700 ease-in-out ${isClosing
          ? 'w-0 opacity-0 -translate-x-full'
          : 'w-2/5 opacity-100 translate-x-0'
          }`}
      >
        <div className="h-full bg-gradient-to-br from-blue-500 via-blue-600 to-blue-700 flex items-center justify-center p-8">
          <div className="text-center text-white">
            <div className="mb-8">
              <div className="w-16 h-16 bg-white/20 rounded-full flex items-center justify-center mx-auto mb-4">
                <svg className="w-8 h-8" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clipRule="evenodd" />
                </svg>
              </div>
              <h1 className="text-4xl font-bold mb-4">GoRTMS</h1>
              <p className="text-blue-100 text-lg">
                Real-Time Messaging System
              </p>
            </div>
            <div className="space-y-4 text-blue-100">
              <div className="flex items-center space-x-3">
                <div className="w-2 h-2 bg-blue-300 rounded-full"></div>
                <span>High-performance messaging</span>
              </div>
              <div className="flex items-center space-x-3">
                <div className="w-2 h-2 bg-blue-300 rounded-full"></div>
                <span>Real-time monitoring</span>
              </div>
              <div className="flex items-center space-x-3">
                <div className="w-2 h-2 bg-blue-300 rounded-full"></div>
                <span>Scalable architecture</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Partie droite - Formulaire avec animation de fade */}
      <div className={`flex-1 flex items-center justify-center bg-gray-50 transition-all duration-700 ${isClosing ? 'w-full opacity-0 scale-95' : 'opacity-100 scale-100'
        }`}>
        <div className={`max-w-md w-full space-y-8 p-8 transition-all duration-700 ${isClosing ? 'opacity-0 translate-y-4' : 'opacity-100 translate-y-0'
          }`}>
          <div className="text-center">
            <h2 className="text-3xl font-extrabold text-gray-900 mb-2">
              Welcome back
            </h2>
            <p className="text-gray-600">
              Sign in to your account
            </p>
          </div>

          <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
                {error}
              </div>
            )}

            <div className="space-y-4">
              <div>
                <label htmlFor="username" className="block text-sm font-medium text-gray-700 mb-1">
                  Username
                </label>
                <input
                  id="username"
                  type="text"
                  required
                  className="w-full px-3 py-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
                  placeholder="Enter your username"
                  value={credentials.username}
                  onChange={handleInputChange('username')}
                />
              </div>
              <div>
                <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">
                  Password
                </label>
                <input
                  id="password"
                  type="password"
                  required
                  className="w-full px-3 py-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
                  placeholder="Enter your password"
                  value={credentials.password}
                  onChange={handleInputChange('password')}
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading || isClosing}
              className="w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-lg text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? (
                <div className="flex items-center space-x-2">
                  <div className="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
                  <span>Signing in...</span>
                </div>
              ) : (
                'Sign in'
              )}
            </button>

            <div className="text-center">
              <button
                type="button"
                onClick={() => setShowRequestModal(true)}
                className="text-sm text-blue-600 hover:text-blue-500 font-medium transition-colors"
              >
                Don't have an account? Request one
              </button>
            </div>
          </form>

          <AccountRequestModal
            isOpen={showRequestModal}
            onClose={() => setShowRequestModal(false)}
          />
        </div>
      </div>
    </div >
  );
};

export default Login;
