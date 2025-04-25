import React, { useState, useEffect } from 'react';
import {
  LayoutDashboard,
  Database,
  MessageSquare,
  GitBranch,
  Settings,
  ChevronDown,
  Menu,
  X,
  Bell,
  Search,
  Activity,
  LogOut,
  User,
} from 'lucide-react';

// Importer les composants
import Dashboard from './pages/Dashboard';
import DomainsManager from './components/DomainsManager';
import QueuesManager from './components/QueuesManager';
import QueueMonitor from './components/QueueMonitor';
import MessagePublisher from './components/MessagePublisher';
import Routing from './pages/Routing';
import Events from './pages/Events';
import api from './api';

// Composant Header
const Header = ({ toggleSidebar }) => {
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);

  return (
    <header className="bg-white border-b border-gray-200">
      <div className="px-4 sm:px-6 lg:px-8 flex justify-between h-16">
        <div className="flex items-center">
          <button
            className="p-2 rounded-md text-gray-500 lg:hidden"
            onClick={toggleSidebar}
          >
            <Menu className="h-6 w-6" />
          </button>
          <div className="ml-4 lg:ml-0 flex items-center">
            <span className="text-lg font-bold text-indigo-600">GoRTMS</span>
          </div>
        </div>

        <div className="flex items-center space-x-4">
          <div className="hidden sm:block relative">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
              <Search className="h-5 w-5 text-gray-400" />
            </div>
            <input
              type="text"
              placeholder="Search..."
              className="pl-10 w-64 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500"
            />
          </div>

          <div className="relative">
            <button
              className="p-2 rounded-full text-gray-500 hover:bg-gray-100 relative"
              onClick={() => setNotificationsOpen(!notificationsOpen)}
            >
              <Bell className="h-6 w-6" />
              <span className="absolute top-0 right-0 block h-2 w-2 rounded-full bg-red-500"></span>
            </button>

            {notificationsOpen && (
              <div className="absolute right-0 mt-2 w-80 bg-white rounded-md shadow-lg overflow-hidden z-10">
                <div className="px-4 py-2 border-b border-gray-200">
                  <h3 className="text-sm font-medium text-gray-700">Notifications</h3>
                </div>
                <div className="divide-y divide-gray-200 max-h-96 overflow-y-auto">
                  <div className="px-4 py-3 hover:bg-gray-50">
                    <div className="flex items-start">
                      <div className="flex-shrink-0 bg-blue-100 rounded-full p-1">
                        <Activity className="h-4 w-4 text-blue-600" />
                      </div>
                      <div className="ml-3">
                        <p className="text-sm text-gray-900">Queue <span className="font-medium">orders.processing</span> is approaching capacity</p>
                        <p className="text-xs text-gray-500">5 minutes ago</p>
                      </div>
                    </div>
                  </div>
                  <div className="px-4 py-3 hover:bg-gray-50">
                    <div className="flex items-start">
                      <div className="flex-shrink-0 bg-green-100 rounded-full p-1">
                        <Database className="h-4 w-4 text-green-600" />
                      </div>
                      <div className="ml-3">
                        <p className="text-sm text-gray-900">New domain <span className="font-medium">analytics</span> created</p>
                        <p className="text-xs text-gray-500">1 hour ago</p>
                      </div>
                    </div>
                  </div>
                </div>
                <div className="px-4 py-2 bg-gray-50 text-xs text-center text-gray-500">
                  <button className="text-indigo-600 hover:text-indigo-800">View all notifications</button>
                </div>
              </div>
            )}
          </div>

          <div className="relative">
            <button
              className="flex items-center space-x-2 text-gray-700 hover:text-gray-900"
              onClick={() => setProfileOpen(!profileOpen)}
            >
              <div className="h-8 w-8 rounded-full bg-indigo-200 flex items-center justify-center">
                <User className="h-5 w-5 text-indigo-600" />
              </div>
              <span className="hidden md:block text-sm font-medium">Admin User</span>
              <ChevronDown className="h-4 w-4 text-gray-500" />
            </button>

            {profileOpen && (
              <div className="absolute right-0 mt-2 w-48 bg-white rounded-md shadow-lg overflow-hidden z-10">
                <div className="py-1">
                  <button className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 w-full text-left">
                    <User className="h-4 w-4 mr-2" />
                    Your Profile
                  </button>
                  <button className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 w-full text-left">
                    <Settings className="h-4 w-4 mr-2" />
                    Settings
                  </button>
                  <button className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 w-full text-left">
                    <LogOut className="h-4 w-4 mr-2" />
                    Sign out
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </header>
  );
};

// Composant Sidebar
const Sidebar = ({ isOpen, toggleSidebar, setPage, currentPage }) => {
  return (
    <>
      {/* Mobile sidebar overlay */}
      {isOpen && (
        <div
          className="fixed inset-0 bg-gray-600 bg-opacity-75 z-20 lg:hidden"
          onClick={toggleSidebar}
        ></div>
      )}

      {/* Sidebar */}
      <div className={`
        fixed inset-y-0 left-0 w-64 bg-white border-r border-gray-200 z-30
        transform transition-transform duration-300 ease-in-out
        ${isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
      `}>
        <div className="h-16 flex items-center justify-between px-4 border-b border-gray-200 lg:hidden">
          <div className="flex items-center">
            <span className="text-lg font-bold text-indigo-600">GoRTMS</span>
          </div>
          <button
            className="p-2 rounded-md text-gray-500"
            onClick={toggleSidebar}
          >
            <X className="h-6 w-6" />
          </button>
        </div>

        <div className="overflow-y-auto h-full pb-8">
          <nav className="px-2 py-4 space-y-1">
            <button
              onClick={() => setPage({ type: 'dashboard' })}
              className={`flex items-center px-4 py-2 text-base font-medium rounded-md w-full text-left
                ${currentPage.type === 'dashboard' ? 'bg-indigo-50 text-indigo-700' : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}
              `}
            >
              <LayoutDashboard className="mr-3 h-5 w-5" />
              Dashboard
            </button>

            <button
              onClick={() => setPage({ type: 'domains' })}
              className={`flex items-center px-4 py-2 text-base font-medium rounded-md w-full text-left
                ${currentPage.type === 'domains' ? 'bg-indigo-50 text-indigo-700' : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}
              `}
            >
              <Database className="mr-3 h-5 w-5" />
              Domains
            </button>

            <button
              onClick={() => setPage({ type: 'domains' })}
              className={`flex items-center px-4 py-2 text-base font-medium rounded-md w-full text-left
                ${(currentPage.type === 'queues' || currentPage.type === 'queue-monitor') ? 'bg-indigo-50 text-indigo-700' : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}
              `}
            >
              <MessageSquare className="mr-3 h-5 w-5" />
              Queues
            </button>

            <button
              onClick={() => setPage({ type: 'routes' })}
              className={`flex items-center px-4 py-2 text-base font-medium rounded-md w-full text-left
                ${currentPage.type === 'routes' ? 'bg-indigo-50 text-indigo-700' : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}
              `}
            >
              <GitBranch className="mr-3 h-5 w-5" />
              Routing
            </button>

            <button
              onClick={() => setPage({ type: 'settings' })}
              className={`flex items-center px-4 py-2 text-base font-medium rounded-md w-full text-left
                ${currentPage.type === 'settings' ? 'bg-indigo-50 text-indigo-700' : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}
              `}
            >
              <Settings className="mr-3 h-5 w-5" />
              Settings
            </button>
          </nav>

          <div className="px-4 py-4 mt-6">
            <div className="bg-indigo-50 rounded-lg px-4 py-3">
              <h3 className="text-sm font-medium text-indigo-800 mb-1">Need help?</h3>
              <p className="text-xs text-indigo-600">
                Check out our documentation or contact support for assistance.
              </p>
              <button className="mt-2 text-xs font-medium text-indigo-700 hover:text-indigo-900 block">
                View Documentation →
              </button>
            </div>
          </div>
        </div>
      </div>
    </>
  );
};

// Composant principal de l'application
const App = () => {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [page, setPage] = useState({ type: 'dashboard' });
  const [systemHealthy, setSystemHealthy] = useState(true);

  const toggleSidebar = () => {
    setSidebarOpen(!sidebarOpen);
  };

  // Vérifier l'état de santé de l'API
  useEffect(() => {
    const checkHealth = async () => {
      try {
        const health = await api.healthCheck();
        setSystemHealthy(health.status === 'ok');
      } catch (err) {
        console.error('Health check failed:', err);
        setSystemHealthy(false);
      }
    };

    checkHealth();
    // Vérifier toutes les 30 secondes
    const interval = setInterval(checkHealth, 30000);

    return () => clearInterval(interval);
  }, []);

  // Navigation vers un domaine ou une file d'attente
  const handleSelectDomain = (domainName) => {
    setPage({ type: 'queues', domainName });
  };

  const handleSelectQueue = (queueName) => {
    setPage({
      type: 'queue-monitor',
      domainName: page.domainName,
      queueName
    });
  };

  const handleBackToDomains = () => {
    setPage({ type: 'domains' });
  };

  const handleBackToQueues = () => {
    setPage({ type: 'queues', domainName: page.domainName });
  };

  const handleBackToDashboard = () => setPage({ type: 'dashboard' });

  const handlePublishMessage = (queueName) => {
    setPage({
      type: 'message-publisher',
      domainName: page.domainName,
      queueName
    });
  };

  const handleViewRouting = (domainName) => {
    setPage({ type: 'domain-routing', domainName });
  };

  // Fonction pour rendre la page active
  const renderPage = () => {
    switch (page.type) {
      case 'domains':
        return (
          <div className="p-6">
            <DomainsManager onSelectDomain={handleSelectDomain} />
          </div>
        );
      case 'queues':
        return (
          <div className="p-6">
            <QueuesManager
              domainName={page.domainName}
              onBack={handleBackToDomains}
              onSelectQueue={handleSelectQueue}
              onPublishMessage={handlePublishMessage}
              onViewRouting={handleViewRouting}
            />
          </div>
        );
      case 'queue-monitor':
        return (
          <div className="p-6">
            <QueueMonitor
              domainName={page.domainName}
              queueName={page.queueName}
              onBack={handleBackToQueues}
            />
          </div>
        );
      case 'routes':
        return (
          <div className="p-6">
            <Routing />
          </div>
        );
      case 'settings':
        return (
          <div className="p-6">
            <h1 className="text-2xl font-bold">Settings</h1>
            <p className="mt-2 text-gray-600">System settings will be available soon.</p>
          </div>
        );
      case 'message-publisher':
        return (
          <div className="p-6">
            <MessagePublisher
              domainName={page.domainName}
              queueName={page.queueName}
              onBack={handleBackToQueues}
              onMessagePublished={() => { }}
            />
          </div>
        );
      case 'domain-routing':
        return (
          <div className="p-6">
            <Routing
              domainName={page.domainName}
              onBack={handleBackToQueues}
            />
          </div>
        );
      case 'events':
        return (
          <div className="p-6">
            <Events onBack={handleBackToDashboard} />
          </div>
        );
      case 'dashboard':
      default:
        return (
          <Dashboard setPage={setPage} />
        );
    }
  };

  return (
    <div className="h-screen flex flex-col">
      <Header toggleSidebar={toggleSidebar} />

      {!systemHealthy && (
        <div className="bg-red-600 text-white px-4 py-2 text-center">
          Backend API unavailable. Some features might not work correctly.
        </div>
      )}

      <div className="flex-1 flex overflow-hidden">
        <Sidebar
          isOpen={sidebarOpen}
          toggleSidebar={toggleSidebar}
          setPage={setPage}
          currentPage={page}
        />

        <main className="flex-1 overflow-auto lg:ml-64">
          {renderPage()}
        </main>
      </div>
    </div>
  );
};

export default App;