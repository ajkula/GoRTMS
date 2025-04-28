// src/pages/Events.js
import React, { useState, useEffect } from 'react';
import { ArrowLeft, Filter, Search, RefreshCw, Loader, AlertTriangle } from 'lucide-react';
import api from '../api';
import { formatEventMessage, formatRelativeTime } from '../utils/utils';

const Events = ({ onBack }) => {
  // Pas de useNavigate, utilisation de onBack à la place
  const [events, setEvents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [filterType, setFilterType] = useState('all');
  const [searchTerm, setSearchTerm] = useState('');
  
  // Événements par page
  const EVENTS_PER_PAGE = 10;
  
  const fetchEvents = async () => {
    try {
      setLoading(true);
      
      // Récupérer les données de l'API
      const statsData = await api.getStats();
      
      if (!statsData || !statsData.recentEvents) {
        setEvents([]);
        setTotalPages(1);
        return;
      }
      
      let allEvents = [...statsData.recentEvents];
      
      // Filtrer par type
      if (filterType !== 'all') {
        allEvents = allEvents.filter(event => event.type === filterType);
      }
      
      // Filtrer par terme de recherche
      if (searchTerm) {
        allEvents = allEvents.filter(event => 
          (event.eventType && event.eventType.toLowerCase().includes(searchTerm.toLowerCase())) || 
          (event.resource && event.resource.toLowerCase().includes(searchTerm.toLowerCase()))
        );
      }
      
      // Formater tous les événements
      const formattedEvents = allEvents
      .sort((a, b) => b.timestamp - a.timestamp)
      .map(event => ({
        id: event.id,
        type: event.type || 'info',
        message: formatEventMessage(event),
        time: formatRelativeTime(event.timestamp),
        resource: event.resource,
        data: event.data,
        timestamp: event.timestamp,
        eventType: event.eventType,
        rawEvent: event
      }));
      
      // Calculer le nombre total de pages
      setTotalPages(Math.ceil(formattedEvents.length / EVENTS_PER_PAGE));
      
      // Paginer les résultats
      const start = (currentPage - 1) * EVENTS_PER_PAGE;
      const paginatedEvents = formattedEvents.slice(start, start + EVENTS_PER_PAGE);
      
      setEvents(paginatedEvents);
    } catch (err) {
      console.error('Error fetching events:', err);
      setError(err.message || 'Failed to load events');
    } finally {
      setLoading(false);
    }
  };
  
  useEffect(() => {
    fetchEvents();
  }, [currentPage, filterType, searchTerm]);
  
  return (
    <div>
      <div className="flex items-center mb-6">
        <button
          onClick={onBack}
          className="mr-3 inline-flex items-center justify-center p-2 rounded-md text-gray-500 hover:bg-gray-100"
        >
          <ArrowLeft className="h-5 w-5" />
        </button>
        <h1 className="text-2xl font-bold">System Events</h1>
      </div>
      
      {/* Filtres et recherche */}
      <div className="bg-white p-4 rounded-lg shadow-sm mb-6 flex flex-wrap items-center gap-4">
        <div className="flex items-center">
          <Filter className="h-5 w-5 text-gray-400 mr-2" />
          <select
            value={filterType}
            onChange={(e) => {
              setFilterType(e.target.value);
              setCurrentPage(1); // Réinitialiser à la première page
            }}
            className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          >
            <option value="all">All Types</option>
            <option value="info">Info</option>
            <option value="warning">Warning</option>
            <option value="error">Error</option>
          </select>
        </div>
        
        <div className="flex-1 relative">
          <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
            <Search className="h-5 w-5 text-gray-400" />
          </div>
          <input
            type="text"
            placeholder="Search events..."
            value={searchTerm}
            onChange={(e) => {
              setSearchTerm(e.target.value);
              setCurrentPage(1); // Réinitialiser à la première page
            }}
            className="pl-10 w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
        
        <button
          onClick={fetchEvents}
          className="inline-flex items-center px-3 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
        >
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </button>
      </div>
      
      {/* Liste des événements */}
      <div className="bg-white rounded-lg shadow-sm overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center h-64">
            <Loader className="h-6 w-6 animate-spin text-indigo-600" />
            <span className="ml-2">Loading events...</span>
          </div>
        ) : error ? (
          <div className="p-6 text-center">
            <AlertTriangle className="h-8 w-8 text-red-500 mx-auto mb-2" />
            <h3 className="text-lg font-medium text-red-800">Failed to load events</h3>
            <p className="text-sm text-red-600 mt-1">{error}</p>
            <button
              onClick={fetchEvents}
              className="mt-3 bg-red-100 px-3 py-1 rounded-md text-red-800 hover:bg-red-200"
            >
              Retry
            </button>
          </div>
        ) : (
          <>
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Type
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Resource
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Message
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Time
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {events.length === 0 ? (
                  <tr>
                    <td colSpan="4" className="px-6 py-12 text-center text-gray-500">
                      No events found matching your criteria
                    </td>
                  </tr>
                ) : (
                  events.map((event) => (
                    <tr key={event.id} className="hover:bg-gray-50">
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium 
                          ${event.type === 'error' ? 'bg-red-100 text-red-800' : 
                            event.type === 'warning' ? 'bg-yellow-100 text-yellow-800' : 
                            'bg-blue-100 text-blue-800'}`}>
                          {event.type}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {event.resource}
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-500">
                        {event.message}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {event.time}
                        <span className="text-xs text-gray-400 block">
                          {new Date(event.timestamp * 1000).toLocaleString()}
                        </span>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
            
            {/* Pagination */}
            {totalPages > 1 && (
              <div className="px-6 py-3 flex items-center justify-between border-t border-gray-200">
                <div className="flex-1 flex justify-between sm:hidden">
                  <button
                    onClick={() => setCurrentPage(p => Math.max(p - 1, 1))}
                    disabled={currentPage === 1}
                    className={`relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md ${
                      currentPage === 1 ? 'text-gray-300' : 'text-gray-700 hover:bg-gray-50'
                    }`}
                  >
                    Previous
                  </button>
                  <button
                    onClick={() => setCurrentPage(p => Math.min(p + 1, totalPages))}
                    disabled={currentPage === totalPages}
                    className={`relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md ${
                      currentPage === totalPages ? 'text-gray-300' : 'text-gray-700 hover:bg-gray-50'
                    }`}
                  >
                    Next
                  </button>
                </div>
                <div className="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
                  <div>
                    <p className="text-sm text-gray-700">
                      Showing <span className="font-medium">{Math.min((currentPage - 1) * EVENTS_PER_PAGE + 1, events.length)}</span> to{' '}
                      <span className="font-medium">{Math.min(currentPage * EVENTS_PER_PAGE, events.length)}</span> of{' '}
                      <span className="font-medium">{events.length}</span> results
                    </p>
                  </div>
                  <div>
                    <nav className="relative z-0 inline-flex rounded-md shadow-sm -space-x-px" aria-label="Pagination">
                      {/* Pages */}
                      {Array.from({ length: Math.min(totalPages, 5) }, (_, i) => {
                        // Afficher jusqu'à 5 pages maximum
                        let pageNum;
                        if (totalPages <= 5) {
                          pageNum = i + 1;
                        } else {
                          // Pour plus de 5 pages, afficher intelligemment les pages autour de la page courante
                          const startPage = Math.max(1, currentPage - 2);
                          pageNum = startPage + i;
                          if (pageNum > totalPages) return null;
                        }
                        
                        return (
                          <button
                            key={pageNum}
                            onClick={() => setCurrentPage(pageNum)}
                            className={`relative inline-flex items-center px-4 py-2 border text-sm font-medium ${
                              currentPage === pageNum
                                ? 'z-10 bg-indigo-50 border-indigo-500 text-indigo-600'
                                : 'bg-white border-gray-300 text-gray-500 hover:bg-gray-50'
                            }`}
                          >
                            {pageNum}
                          </button>
                        );
                      }).filter(Boolean)}
                    </nav>
                  </div>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
};

export default Events;
