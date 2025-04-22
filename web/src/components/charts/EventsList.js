import React from 'react';
import { formatEventMessage, formatRelativeTime } from '../../utils/utils';

const AlertItem = ({ alert }) => {
  const typeColors = {
    info: 'bg-blue-100 text-blue-800',
    warning: 'bg-yellow-100 text-yellow-800',
    error: 'bg-red-100 text-red-800'
  };

  const color = typeColors[alert.type] || typeColors.info;

  return (
    <div className="border-b border-gray-200 py-3 last:border-0">
      <div className="flex items-start">
        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${color}`}>
          {alert.type}
        </span>
        <div className="ml-3">
          <p className="text-sm text-gray-900">{alert.message}</p>
          <p className="mt-1 text-xs text-gray-500">{alert.time}</p>
        </div>
      </div>
    </div>
  );
};

const EventsList = ({ events, setPage }) => {
  // Formater les événements pour l'affichage
  const formattedEvents = events.map(event => {
    // Si l'événement est déjà formaté, l'utiliser tel quel
    if (event.message && event.time) {
      return event;
    }
    
    // Sinon, formater l'événement
    return {
      id: event.id,
      type: event.type || 'info',
      message: formatEventMessage(event),
      time: formatRelativeTime(event.timestamp),
      rawEvent: event
    };
  });

  return (
    <div>
      <div className="px-6 py-4 border-b border-gray-200">
        <h3 className="text-lg font-medium text-gray-900">Recent Events</h3>
      </div>

      {formattedEvents.length > 0 ? (
        <div className="px-6 divide-y divide-gray-200">
          {formattedEvents.map(alert => (
            <AlertItem key={alert.id} alert={alert} />
          ))}
        </div>
      ) : (
        <div className="px-6 py-6 text-center text-gray-500">
          <p>No recent events to display.</p>
        </div>
      )}

      <div className="px-6 py-3 bg-gray-50 text-right">
        <button
          onClick={() => setPage({ type: 'events' })}
          className="text-sm font-medium text-indigo-600 hover:text-indigo-500"
        >
          View all events
        </button>
      </div>
    </div>
  );
};

export default EventsList;
