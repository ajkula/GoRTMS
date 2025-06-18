import React, { useState, useEffect } from 'react';
import { useHealthCheck } from './hooks/useHealthCheck';
import { useNavigation } from './hooks/useNavigation';
import { useAuth } from './hooks/useAuth';
import { AuthGuard } from './components/auth/AuthGuard';
import { Header } from './components/layout/Header';
import { Sidebar } from './components/layout/Sidebar';

// Pages
import Dashboard from './pages/Dashboard';
import DomainsManager from './components/DomainsManager';
import QueuesManager from './components/QueuesManager';
import QueueMonitor from './components/QueueMonitor';
import MessagePublisher from './components/MessagePublisher';
import Routing from './pages/Routing';
import ConsumerGroupsManager from './components/ConsumerGroupsManager';
import ConsumerGroupDetail from './components/ConsumerGroupDetail';
import Settings from './components/SettingsComponent';
import Events from './pages/Events';
import Login from './pages/Login';
import Profile from './pages/Profile';
// import UserManagement from './pages/admin/UserManagement';
// import ServiceAccountManagement from './pages/admin/ServiceAccountManagement';

const App = () => {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [isLoginAnimating, setIsLoginAnimating] = useState(false);
  const { systemHealthy } = useHealthCheck();
  const { page, navigate, setPage } = useNavigation();
  const { isAuthenticated, loading: authLoading } = useAuth();

  const toggleSidebar = () => setSidebarOpen(!sidebarOpen);

  useEffect(() => {
    if (authLoading) return;

    if (!isAuthenticated && page.type !== 'login') {
      navigate.toLogin();
      setIsLoginAnimating(false);
    } else if (isAuthenticated && page.type === 'login') {
      setIsLoginAnimating(true);

      setTimeout(() => {
        navigate.toDashboard();
        setIsLoginAnimating(false);
      }, 800);
    }
  }, [authLoading, isAuthenticated, page.type, navigate]);

  const pageComponents = {
    // 'login': <Login />,
    'dashboard': <AuthGuard><Dashboard setPage={setPage} /></AuthGuard>,
    'domains': (
      <AuthGuard>
        <DomainsManager onSelectDomain={(domainName) => navigate.toQueues(domainName)} />
      </AuthGuard>
    ),
    'queues': (
      <AuthGuard>
        <QueuesManager
          domainName={page.domainName}
          onBack={navigate.toDomains}
          onSelectQueue={(queueName) => navigate.toQueueMonitor(page.domainName, queueName)}
          onPublishMessage={(queueName) => navigate.toMessagePublisher(page.domainName, queueName)}
          onViewRouting={(domainName) => navigate.toRouting(domainName)}
        />
      </AuthGuard>
    ),
    'queue-monitor': (
      <AuthGuard>
        <QueueMonitor
          domainName={page.domainName}
          queueName={page.queueName}
          onBack={() => navigate.toQueues(page.domainName)}
        />
      </AuthGuard>
    ),
    'message-publisher': (
      <AuthGuard>
        <MessagePublisher
          domainName={page.domainName}
          queueName={page.queueName}
          onBack={() => navigate.toQueues(page.domainName)}
          onMessagePublished={() => { }}
        />
      </AuthGuard>
    ),
    'routes': <AuthGuard><Routing /></AuthGuard>,
    'domain-routing': (
      <AuthGuard>
        <Routing
          domainName={page.domainName}
          onBack={() => navigate.toQueues(page.domainName)}
        />
      </AuthGuard>
    ),
    'consumer-groups': (
      <AuthGuard>
        <ConsumerGroupsManager
          onSelectGroup={navigate.toConsumerGroupDetail}
          onBack={navigate.toDashboard}
        />
      </AuthGuard>
    ),
    'consumer-group-detail': (
      <AuthGuard>
        <ConsumerGroupDetail
          domainName={page.domainName}
          queueName={page.queueName}
          groupID={page.groupID}
          onBack={navigate.toConsumerGroups}
        />
      </AuthGuard>
    ),
    'events': <AuthGuard><Events onBack={navigate.toDashboard} /></AuthGuard>,
    'settings': (
      <AuthGuard>
        <Settings />
      </AuthGuard>
    ),
    'profile': (
      <AuthGuard>
        <Profile onBack={navigate.toDashboard} />
      </AuthGuard>
    ),
    'admin-services': (
      <AuthGuard>
        <div className="p-4">
          <h1 className="text-2xl font-bold mb-4">Service Account Management</h1>
          <p>Service account management interface coming soon...</p>
          <button
            onClick={navigate.toDashboard}
            className="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
          >
            Back to Dashboard
          </button>
        </div>
      </AuthGuard>
    ),
    'admin-users': (
      <AuthGuard requiredRole="admin">
        <div className="p-4">
          <h1 className="text-2xl font-bold mb-4">User Management</h1>
          <p>User management interface coming soon...</p>
          <button
            onClick={navigate.toDashboard}
            className="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
          >
            Back to Dashboard
          </button>
        </div>
      </AuthGuard>
    ),
  };

  if (authLoading) {
    return (
      <div className="h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading GoRTMS...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-screen">
      {page.type === 'login' ? (
        <Login isClosing={isLoginAnimating} />
      ) : (
        <div className="h-screen flex flex-col">
          <Header toggleSidebar={toggleSidebar} />

          {!systemHealthy && (
            <div
              className="bg-red-600 text-white px-4 py-2 text-center"
              data-testid="health-indicator"
            >
              Backend API unavailable. Some features might not work correctly.
            </div>
          )}

          <div className="flex-1 flex overflow-hidden">
            <Sidebar
              isOpen={sidebarOpen}
              toggleSidebar={toggleSidebar}
              navigate={navigate}
              currentPage={page}
              setPage={setPage}
            />

            <main className="flex-1 overflow-auto lg:ml-64">
              <div className="p-6">
                {pageComponents[page.type] || pageComponents['dashboard']}
              </div>
            </main>
          </div>
        </div>
      )}
    </div>
  );
};

export default App;
