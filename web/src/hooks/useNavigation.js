import { useState, useMemo } from 'react';

export const useNavigation = () => {
  const [page, setPage] = useState({ type: 'dashboard' });

  const navigate = useMemo(() => ({
    toLogin: () => setPage({ type: 'login' }),
    toDashboard: () => setPage({ type: 'dashboard' }),
    toDomains: () => setPage({ type: 'domains' }),
    toQueues: (domainName) => setPage({ type: 'queues', domainName }),
    toQueueMonitor: (domainName, queueName) => 
      setPage({ type: 'queue-monitor', domainName, queueName }),
    toMessagePublisher: (domainName, queueName) => 
      setPage({ type: 'message-publisher', domainName, queueName }),
    toRouting: (domainName = null) => 
      setPage(domainName ? { type: 'domain-routing', domainName } : { type: 'routes' }),
    toConsumerGroups: () => setPage({ type: 'consumer-groups' }),
    toConsumerGroupDetail: (domainName, queueName, groupID) => 
      setPage({ type: 'consumer-group-detail', domainName, queueName, groupID }),
    toSettings: () => setPage({ type: 'settings' }),
    toEvents: () => setPage({ type: 'events' }),
    toProfile: () => setPage({ type: 'profile' }),
    toUserManagement: () => setPage({ type: 'admin-users' }),
    toServiceManagement: () => setPage({ type: 'admin-services' }),
  }), []);

  return { page, navigate, setPage };
};
