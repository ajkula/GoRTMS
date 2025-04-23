import React, { useState, useEffect } from 'react';
import { RefreshCw, Loader, AlertTriangle } from 'lucide-react';
import api from '../api';

// Import des composants de charts extraits
import StatCards from '../components/charts/StatCards';
import MessageActivityChart from '../components/charts/MessageActivityChart';
import DomainPieChart from '../components/charts/DomainPieChart';
import DomainQueueTreeMap from '../components/charts/DomainQueueTreeMap';
import QueueFillRatesChart from '../components/charts/QueueFillRatesChart';
import ConsumptionRateChart from '../components/charts/ConsumptionRateChart';
import ResourceMonitor from '../components/charts/ResourceMonitor';
import DomainUsageChart from '../components/charts/DomainUsageChart';
import EventsList from '../components/charts/EventsList';

const Dashboard = ({ setPage }) => {
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Fonction pour charger les données du dashboard
  const loadDashboardData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Charger les statistiques générales
      const statsData = await api.getStats();
      setStats(statsData);
    } catch (err) {
      console.error('Error fetching dashboard data:', err);
      setError(err.message || 'Failed to load dashboard data');

      // En cas d'erreur, configurer des données par défaut
      setStats({
        domains: 0,
        queues: 0,
        messages: 0,
        routes: 0,
        messageRates: [],
        activeDomains: [],
        topQueues: [],
        recentEvents: []
      });
    } finally {
      setLoading(false);
    }
  };

  // Charger les données au montage
  useEffect(() => {
    loadDashboardData();

    // Actualiser les données toutes les 30 secondes
    const interval = setInterval(loadDashboardData, 30000);
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

      {/* Stats Cards */}
      <div className="mb-6">
        <StatCards stats={stats} />
      </div>

      {/* Resource Monitor et Domain Usage - Nouveau! */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        <div className="bg-white rounded-lg shadow p-6">
          <DomainPieChart data={stats?.activeDomains || []} />
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <DomainUsageChart />
        </div>
      </div>

      {/* Message Activity Chart - Full Width */}
      <div className="mb-6">
        <div className="bg-white rounded-lg shadow p-6">
          <MessageActivityChart data={stats?.messageRates || []} />
        </div>
      </div>

      {/* Domain Pie Chart and Domain Queue TreeMap */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        <div>
          <ResourceMonitor />
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <DomainQueueTreeMap data={stats?.topQueues || []} />
        </div>
      </div>

      {/* Queue Fill Rates and Consumption Rate */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        <div className="bg-white rounded-lg shadow p-6">
          <QueueFillRatesChart data={stats?.topQueues || []} />
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <ConsumptionRateChart data={stats?.messageRates || []} />
        </div>
      </div>

      {/* Recent Events */}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <EventsList
          events={stats?.recentEvents || []}
          setPage={setPage}
        />
      </div>
    </div>
  );
};

export default Dashboard;