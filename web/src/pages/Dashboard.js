import React, { useState, useEffect } from 'react';
import { PieChart, Pie, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, Cell, ResponsiveContainer } from 'recharts';
import { Activity, MessageSquare, Database, GitBranch, ArrowUpRight, ArrowDownRight, RefreshCw, Loader, AlertTriangle } from 'lucide-react';
import api from '../api';
import { formatEventMessage, formatRelativeTime } from '../utils/utils';

const StatCard = ({ title, value, icon, trend, trendValue }) => {
  const Icon = icon;
  const isTrendUp = trend === 'up';

  return (
    <div className="bg-white rounded-lg shadow p-6 flex flex-col">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-gray-500 text-sm font-medium">{title}</h3>
        <div className="bg-indigo-100 p-2 rounded-lg">
          <Icon className="h-5 w-5 text-indigo-600" />
        </div>
      </div>
      <div className="flex items-baseline">
        <span className="text-3xl font-bold text-gray-900">{value !== undefined ? value.toLocaleString() : '-'}</span>
        {trendValue ? (
          <span className={`ml-2 flex items-center text-sm ${isTrendUp ? 'text-green-500' : 'text-red-500'}`}>
            {isTrendUp ? <ArrowUpRight size={16} /> : <ArrowDownRight size={16} />}
            {Math.floor(trendValue)}%
          </span>
        ) : (
          <span className={`ml-2 flex items-center text-sm`}>
            {0}
          </span>
        )}
      </div>
    </div>
  );
};

const AlertItem = ({ alert }) => {
  const typeColors = {
    info: 'bg-blue-100 text-blue-800',
    warning: 'bg-yellow-100 text-yellow-800',
    error: 'bg-red-100 text-red-800'
  };

  return (
    <div className="border-b border-gray-200 py-3 last:border-0">
      <div className="flex items-start">
        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${typeColors[alert.type]}`}>
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

const Dashboard = ({ setPage }) => {
  const [stats, setStats] = useState(null);
  const [domainsData, setDomainsData] = useState([]);
  const [messageActivity, setMessageActivity] = useState([]);
  const [recentEvents, setRecentEvents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Fonction pour charger les données du dashboard
  const loadDashboardData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Charger les statistiques générales
      const statsData = await api.getStats();
      // DEBUG
      // console.log("API response:", statsData);
      setStats(statsData);

      // Transformer les données de domaines pour le graphique
      if (statsData.activeDomains && statsData.activeDomains.length > 0) {
        const colors = ['#8884d8', '#82ca9d', '#ffc658', '#ff8042', '#0088fe'];
        const domainChartData = statsData.activeDomains.map((domain, index) => ({
          name: domain.name,
          value: domain.messageCount || 0,
          color: colors[index % colors.length]
        }));
        setDomainsData(domainChartData);
      }

      // Créer des données d'activité à partir des taux de messages
      if (statsData.messageRates && statsData.messageRates.length > 0) {
        const activityData = statsData.messageRates.map(rate => {
          const date = new Date(rate.timestamp * 1000);
          const timeString = `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;

          return {
            time: timeString,
            published: rate.publishedTotal || 0,
            consumed: rate.consumedTotal || 0,
          };
        });

        setMessageActivity(activityData);
      }

      if (statsData.recentEvents && statsData.recentEvents.length > 0) {
        const formattedEvents = statsData.recentEvents
          .sort((a, b) => a.time - b.time)
          .map(event => {
            const existingEvent = recentEvents.find(e => e.id === event.id);

            if (existingEvent && existingEvent.rawEvent &&
              existingEvent.rawEvent.timestamp === event.timestamp
            ) {
              return {
                ...existingEvent,
                rawEvent: event,
              };
            }
            return {
              id: event.id,
              type: event.type,
              message: formatEventMessage(event),
              time: formatRelativeTime(event.timestamp),
              // Conserver les données brutes pour les références futures
              rawEvent: event
            }
          });
        setRecentEvents(formattedEvents);
      } else {
        // Toujours avoir des événements d'exemple si aucun n'est disponible
        setRecentEvents([{
          id: 'example-event',
          type: 'info',
          message: 'No recent events available',
          time: 'Just now'
        }]);
      }

    } catch (err) {
      console.error('Error fetching dashboard data:', err);
      setError(err.message || 'Failed to load dashboard data');

      // En cas d'erreur, configurer des données par défaut pour l'UI
      setStats({
        domains: 0,
        queues: 0,
        messages: 0,
        routes: 0
      });
      setDomainsData([]);
      setMessageActivity([]);
    } finally {
      setLoading(false);
    }
  };

  // Charger les données au montage
  useEffect(() => {
    loadDashboardData();

    // Actualiser les données toutes les 30 secondes
    const interval = setInterval(loadDashboardData, 30000);
    if (stats?.recentEvents?.length > 0) {
      setRecentEvents(prevEvents =>
        prevEvents.map(event => ({
          ...event,
          time: event.rawEvent ? formatRelativeTime(event.rawEvent.timestamp) : event.time
        }))
      );
    }

    return () => clearInterval(interval);
  }, []);

  if (loading && !stats) {
    return (
      <div className="flex items-center justify-center h-64 p-6">
        <Loader className="h-8 w-8 animate-spin text-indigo-600" />
        <span className="ml-2">Loading dashboard data...</span>
      </div>
    );
  }

  return (
    <div className="p-6 bg-gray-50 min-h-screen">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <p className="text-gray-600">Monitor and manage your GoRTMS instance</p>
        </div>

        <button
          onClick={loadDashboardData}
          className="inline-flex items-center px-3 py-2 border border-gray-300 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          disabled={loading}
        >
          {loading ? (
            <Loader className="h-4 w-4 animate-spin mr-2" />
          ) : (
            <RefreshCw className="h-4 w-4 mr-2" />
          )}
          Refresh
        </button>
      </div>

      {error && (
        <div className="mb-6 bg-red-50 border-l-4 border-red-500 p-4 rounded">
          <div className="flex">
            <AlertTriangle className="h-5 w-5 text-red-500 mr-2" />
            <div>
              <h3 className="text-sm font-medium text-red-800">Error loading dashboard</h3>
              <p className="text-sm text-red-700 mt-1">{error}</p>
            </div>
          </div>
        </div>
      )}

      {/* Stats Overview */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-6">
        <StatCard
          title="Total Domains"
          value={stats?.domains}
          icon={Database}
          trend={stats?.domainTrend?.direction}
          trendValue={stats?.domainTrend?.value}
        />
        <StatCard
          title="Total Queues"
          value={stats?.queues}
          icon={MessageSquare}
          trend={stats?.queueTrend?.direction}
          trendValue={stats?.queueTrend?.value}
        />
        <StatCard
          title="Total Messages"
          value={stats?.messages}
          icon={Activity}
          trend={stats?.messageTrend?.direction}
          trendValue={stats?.messageTrend?.value}
        />
        <StatCard
          title="Routing Rules"
          value={stats?.routes}
          icon={GitBranch}
          trend={stats?.routeTrend?.direction}
          trendValue={stats?.routeTrend?.value}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Message Activity Chart */}
        <div className="lg:col-span-2 bg-white rounded-lg shadow p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Message Activity</h2>

          {messageActivity.length > 0 ? (
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart
                  data={messageActivity}
                  margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
                >
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="time" />
                  <YAxis />
                  <Tooltip />
                  <Legend />
                  <Bar dataKey="published" fill="#8884d8" name="Published" />
                  <Bar dataKey="consumed" fill="#82ca9d" name="Consumed" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <div className="h-64 flex items-center justify-center text-gray-500">
              <p>No message activity data available.</p>
            </div>
          )}
        </div>

        {/* Domain Distribution */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Messages by Domain</h2>

          {domainsData.length > 0 ? (
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={domainsData}
                    cx="50%"
                    cy="50%"
                    labelLine={false}
                    outerRadius={80}
                    fill="#8884d8"
                    dataKey="value"
                    label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                  >
                    {domainsData.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <div className="h-64 flex items-center justify-center text-gray-500">
              <p>No domain data available.</p>
            </div>
          )}
        </div>

        {/* Recent Alerts */}
        <div className="lg:col-span-3 bg-white rounded-lg shadow overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">Recent Events</h3>
          </div>

          {recentEvents.length > 0 ? (
            <div className="px-6 divide-y divide-gray-200">
              {recentEvents.map(alert => (
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
      </div>
    </div>
  );
};

export default Dashboard;