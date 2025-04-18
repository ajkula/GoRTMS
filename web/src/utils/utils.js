

  // Fonction pour formater un message d'événement basé sur son type
  export const formatEventMessage = (event) => {
    switch (event.eventType) {
      case 'domain_created':
        return `Domain "${event.resource}" created`;
      case 'domain_deleted':
        return `Domain "${event.resource}" deleted`;
      case 'queue_created':
        return `Queue "${event.resource}" created`;
      case 'queue_deleted':
        return `Queue "${event.resource}" deleted`;
      case 'queue_capacity':
        return `Queue "${event.resource}" approaching capacity (${Math.round(event.data)}%)`;
      case 'connection_lost':
        return `Connection lost to consumer on "${event.resource}"`;
      case 'domain_active':
        let queueCount = event.data?.queueCount || 0;
        return `Domain "${event.resource}" is active with ${queueCount} queues`;
      case 'routing_rule_created':
        return `Routing rule created from "${event.data.source}" to "${event.data.destination}" in ${event.resource}`;
      default:
        return `Event on "${event.resource}": ${JSON.stringify(event.data)}`;
    }
  };

  // Fonction pour calculer le temps relatif
  export const formatRelativeTime = (timestamp) => {
    const now = Math.floor(Date.now() / 1000);
    const diff = now - timestamp;

    if (diff < 60) {
      return 'Just now';
    } else if (diff < 3600) {
      const minutes = Math.floor(diff / 60);
      return `${minutes} min ago`;
    } else if (diff < 86400) {
      const hours = Math.floor(diff / 3600);
      return `${hours} hour${hours > 1 ? 's' : ''} ago`;
    } else {
      const days = Math.floor(diff / 86400);
      return `${days} day${days > 1 ? 's' : ''} ago`;
    }
  };