import React, { useState } from 'react';
import { useHealthCheck } from './hooks/useHealthCheck';
import { useNavigation } from './hooks/useNavigation';
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
import Events from './pages/Events';

const App = () => {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const { systemHealthy } = useHealthCheck();
  const { page, navigate, setPage } = useNavigation();

  const toggleSidebar = () => setSidebarOpen(!sidebarOpen);

  // Mapping des pages vers les composants
  const pageComponents = {
    'dashboard': <Dashboard setPage={setPage} />,
    
    'domains': (
      <DomainsManager onSelectDomain={(domainName) => navigate.toQueues(domainName)} />
    ),
    
    'queues': (
      <QueuesManager
        domainName={page.domainName}
        onBack={navigate.toDomains}
        onSelectQueue={(queueName) => navigate.toQueueMonitor(page.domainName, queueName)}
        onPublishMessage={(queueName) => navigate.toMessagePublisher(page.domainName, queueName)}
        onViewRouting={(domainName) => navigate.toRouting(domainName)}
      />
    ),
    
    'queue-monitor': (
      <QueueMonitor
        domainName={page.domainName}
        queueName={page.queueName}
        onBack={() => navigate.toQueues(page.domainName)}
      />
    ),
    
    'message-publisher': (
      <MessagePublisher
        domainName={page.domainName}
        queueName={page.queueName}
        onBack={() => navigate.toQueues(page.domainName)}
        onMessagePublished={() => {}}
      />
    ),
    
    'routes': <Routing />,
    
    'domain-routing': (
      <Routing
        domainName={page.domainName}
        onBack={() => navigate.toQueues(page.domainName)}
      />
    ),
    
    'consumer-groups': (
      <ConsumerGroupsManager
        onSelectGroup={navigate.toConsumerGroupDetail}
        onBack={navigate.toDashboard}
      />
    ),
    
    'consumer-group-detail': (
      <ConsumerGroupDetail
        domainName={page.domainName}
        queueName={page.queueName}
        groupID={page.groupID}
        onBack={navigate.toConsumerGroups}
      />
    ),
    
    'events': <Events onBack={navigate.toDashboard} />,
    
    'settings': (
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="mt-2 text-gray-600">System settings will be available soon.</p>
      </div>
    ),
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
  );
};

export default App;