

// Fonction pour formater un message d'événement basé sur son type
export const formatEventMessage = (event) => {
  switch (event.eventType) {
    case 'domain_created':
      return `Domain "${event.resource}" created`;
    case 'domain_deleted':
      return `Domain "${event.resource}" deleted`;
    case 'domain_active':
      let queueCount = event.data?.queueCount || 0;
      return `Domain "${event.resource}" is active with ${queueCount} queues`;
    case 'queue_created':
      return `Queue "${event.resource}" created`;
    case 'queue_deleted':
      return `Queue "${event.resource}" deleted`;
    case 'queue_capacity':
      return `Queue "${event.resource}" approaching capacity (${Math.round(event.data)}%)`;
    case 'routing_rule_created':
      return `Routing rule created from "${event.data.source}" to "${event.data.destination}" in ${event.resource}`;
    case 'connection_lost':
      return `Connection lost to consumer on "${event.resource}"`;
    default:
      return `"${event.resource}": ${JSON.stringify(event.data)}`;
  }
};

// Fonction pour calculer le temps relatif
export const formatRelativeTime = (timestamp) => {
  if (!timestamp) return 'Unknown time';

  const now = Math.floor(Date.now() / 1000);
  const secondsAgo = now - timestamp;

  if (secondsAgo < 5) return 'Just now';
  if (secondsAgo < 60) return `${secondsAgo} seconds ago`;
  if (secondsAgo < 120) return '1 minute ago';
  if (secondsAgo < 3600) return `${Math.floor(secondsAgo / 60)} minutes ago`;
  if (secondsAgo < 7200) return '1 hour ago';
  if (secondsAgo < 86400) return `${Math.floor(secondsAgo / 3600)} hours ago`;
  if (secondsAgo < 172800) return '1 day ago';

  // Pour les dates plus anciennes, utiliser la date formatée
  const date = new Date(timestamp * 1000);
  return date.toLocaleDateString();
};

// Fonction pour formater les nombres avec des séparateurs de milliers
export const formatNumber = (number) => {
  if (number === undefined || number === null) return '-';
  return number.toLocaleString();
};

// Fonction pour générer une couleur à partir d'une chaîne (pour les graphiques)
export const stringToColor = (str) => {
  if (!str) return '#8884d8'; // Couleur par défaut

  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }

  let color = '#';
  for (let i = 0; i < 3; i++) {
    const value = (hash >> (i * 8)) & 0xFF;
    color += ('00' + value.toString(16)).substr(-2);
  }

  return color;
};
