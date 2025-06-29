// Function to format an event message based on its type
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

// Function to calculate relative time
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

  // For older dates, use the formatted date
  const date = new Date(timestamp * 1000);
  return date.toLocaleDateString();
};

export const formatNumber = (number) => {
  if (number === undefined || number === null) return '-';
  return number.toLocaleString();
};

// Function to generate a color from a string
export const stringToColor = (str) => {
  if (!str) return '#8884d8';

  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }

  let color = '#';
  for (let i = 0; i < 3; i++) {
    const value = (hash >> (i * 8)) & 0xFF;
    color += ('00' + value.toString(8)).substr(-2);
  }

  return color;
};

export const formatDuration = (nanoseconds) => {
  if (!nanoseconds || nanoseconds <= 0) {
    return 'No TTL';
  }

  // nanosecondes to millisecondes
  const ms = nanoseconds / 1000000;

  // Time units
  const seconds = Math.floor(ms / 1000) % 60;
  const minutes = Math.floor(ms / (1000 * 60)) % 60;
  const hours = Math.floor(ms / (1000 * 60 * 60)) % 24;
  const days = Math.floor(ms / (1000 * 60 * 60 * 24));

  const timeQualifiers = {
    0: 'd',
    1: 'h',
    2: 'm',
    3: 's',
  };

  // build chain
  return [days, hours, minutes, seconds]
    .reduce((prev, curr, idx) => {
      if (curr > 0) {
        prev.push(curr + timeQualifiers[idx])
      }
      return prev
    }, []).join(' ');
}
