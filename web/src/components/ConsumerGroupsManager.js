import React, { useState, useEffect } from 'react';
import { PlusCircle, Trash2, Clock, RefreshCw, AlertTriangle, Settings, ArrowRight } from 'lucide-react';
import api from '../api';
import { formatDuration } from '../utils/utils';

const ConsumerGroupsManager = ({ onSelectGroup, onBack }) => {
  const [domains, setDomains] = useState([]);
  const [consumerGroups, setConsumerGroups] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newGroupForm, setNewGroupForm] = useState({
    domainName: '',
    queueName: '',
    groupID: '',
    ttl: '1h'
  });

  // Charger tous les domaines
  useEffect(() => {
    const fetchDomains = async () => {
      try {
        const domainsData = await api.getDomains();
        setDomains(domainsData);
      } catch (err) {
        console.error('Error fetching domains:', err);
        setError(err.message || 'Failed to load domains');
      }
    };

    fetchDomains();
  }, []);

  // Fonction pour charger tous les consumer groups de tous les domaines
  const fetchAllConsumerGroups = async () => {
    try {
      setLoading(true);
      setError(null);
      
      // Cette API doit être créée côté backend
      const data = await api.getAllConsumerGroups();
      setConsumerGroups(data.groups || []);
    } catch (err) {
      console.error('Error fetching consumer groups:', err);
      setError(err.message || 'Failed to load consumer groups');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAllConsumerGroups();
    
    // Rafraîchir périodiquement
    const interval = setInterval(fetchAllConsumerGroups, 30000);
    return () => clearInterval(interval);
  }, []);

  // Gérer la création d'un nouveau groupe
  const handleCreateGroup = async (e) => {
    e.preventDefault();
    
    try {
      await api.createConsumerGroup(
        newGroupForm.domainName,
        newGroupForm.queueName,
        {
          groupID: newGroupForm.groupID,
          ttl: newGroupForm.ttl
        }
      );
      
      setShowCreateModal(false);
      setNewGroupForm({
        domainName: '',
        queueName: '',
        groupID: '',
        ttl: '1h'
      });
      
      await fetchAllConsumerGroups();
    } catch (err) {
      setError(err.message || 'Failed to create consumer group');
    }
  };

  // Gérer la suppression d'un groupe
  const handleDeleteGroup = async (domainName, queueName, groupID) => {
    if (!window.confirm(`Are you sure you want to delete consumer group "${groupID}"?`)) {
      return;
    }

    try {
      await api.deleteConsumerGroup(domainName, queueName, groupID);
      await fetchAllConsumerGroups();
    } catch (err) {
      setError(err.message || 'Failed to delete consumer group');
    }
  };

  // Chargement des queues pour un domaine sélectionné
  const [queues, setQueues] = useState([]);
  useEffect(() => {
    const fetchQueues = async () => {
      if (!newGroupForm.domainName) {
        setQueues([]);
        return;
      }
      
      try {
        const queuesData = await api.getQueues(newGroupForm.domainName);
        setQueues(queuesData);
      } catch (err) {
        console.error(`Error fetching queues for ${newGroupForm.domainName}:`, err);
      }
    };
    
    fetchQueues();
  }, [newGroupForm.domainName]);

  return (
    <div className="container mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Consumer Groups</h1>
        <div>
          <button
            onClick={() => setShowCreateModal(true)}
            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            <PlusCircle className="h-5 w-5 mr-2" />
            New Consumer Group
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border-l-4 border-red-400 p-4 mb-4">
          <div className="flex">
            <AlertTriangle className="h-5 w-5 text-red-400" />
            <div className="ml-3">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          </div>
        </div>
      )}

      {/* Tableau des consumer groups */}
      <div className="bg-white shadow overflow-hidden sm:rounded-md mb-8">
        <div className="px-4 py-5 sm:px-6 flex justify-between items-center">
          <h2 className="text-lg leading-6 font-medium text-gray-900">All Consumer Groups</h2>
          <button 
            onClick={fetchAllConsumerGroups}
            className="inline-flex items-center px-3 py-1 border border-gray-300 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
          >
            <RefreshCw className="h-4 w-4 mr-1" />
            Refresh
          </button>
        </div>
        {loading ? (
          <div className="px-4 py-5 text-center text-gray-500">
            <RefreshCw className="h-8 w-8 animate-spin mx-auto mb-2" />
            Loading consumer groups...
          </div>
        ) : consumerGroups.length === 0 ? (
          <div className="px-4 py-5 text-center text-gray-500">
            No consumer groups found. Create your first group to get started.
          </div>
        ) : (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Domain
                </th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Queue
                </th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Group ID
                </th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Consumers
                </th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Last Activity
                </th>
                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  TTL
                </th>
                <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {consumerGroups.map((group) => {
                console.log({ group });
                return (
                <tr key={`${group.DomainName}-${group.QueueName}-${group.GroupID}`}>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                    {group.DomainName}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {group.QueueName}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {group.GroupID}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {group.ConsumerIDs.length}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {new Date(group.LastActivity).toLocaleString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDuration(group.TTL)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button
                      onClick={() => onSelectGroup(group.DomainName, group.QueueName, group.GroupID)}
                      className="text-indigo-600 hover:text-indigo-900 mr-3"
                    >
                      <Settings className="h-4 w-4 inline mr-1" />
                      Configure
                    </button>
                    <button
                      onClick={() => handleDeleteGroup(group.DomainName, group.QueueName, group.GroupID)}
                      className="text-red-600 hover:text-red-900"
                    >
                      <Trash2 className="h-4 w-4 inline" />
                    </button>
                  </td>
                </tr>
              )})}
            </tbody>
          </table>
        )}
      </div>

      {/* Modal de création */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-96 max-w-full">
            <h3 className="text-lg font-medium mb-4">Create New Consumer Group</h3>
            <form onSubmit={handleCreateGroup}>
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Domain
                </label>
                <select
                  value={newGroupForm.domainName}
                  onChange={(e) => setNewGroupForm({...newGroupForm, domainName: e.target.value})}
                  className="w-full border border-gray-300 rounded-md p-2"
                  required
                >
                  <option value="">Select Domain</option>
                  {domains.map(domain => (
                    <option key={domain.name} value={domain.name}>{domain.name}</option>
                  ))}
                </select>
              </div>

              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Queue
                </label>
                <select
                  value={newGroupForm.queueName}
                  onChange={(e) => setNewGroupForm({...newGroupForm, queueName: e.target.value})}
                  className="w-full border border-gray-300 rounded-md p-2"
                  required
                  disabled={!newGroupForm.domainName}
                >
                  <option value="">Select Queue</option>
                  {queues.map(queue => (
                    <option key={queue.name} value={queue.name}>{queue.name}</option>
                  ))}
                </select>
              </div>

              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Group ID
                </label>
                <input
                  type="text"
                  value={newGroupForm.groupID}
                  onChange={(e) => setNewGroupForm({...newGroupForm, groupID: e.target.value})}
                  className="w-full border border-gray-300 rounded-md p-2"
                  placeholder="Enter group ID"
                  required
                />
              </div>

              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  TTL (Time to Live)
                </label>
                <div className="flex space-x-2">
                  <input
                    type="text"
                    value={newGroupForm.ttl}
                    onChange={(e) => setNewGroupForm({...newGroupForm, ttl: e.target.value})}
                    className="w-full border border-gray-300 rounded-md p-2"
                    placeholder="e.g. 30m, 1h, 24h"
                  />
                  <select
                    onChange={(e) => setNewGroupForm({...newGroupForm, ttl: e.target.value})}
                    className="border border-gray-300 rounded-md p-2"
                  >
                    <option value="5m">5m</option>
                    <option value="15m">15m</option>
                    <option value="30m">30m</option>
                    <option value="1h">1h</option>
                    <option value="4h">4h</option>
                    <option value="12h">12h</option>
                    <option value="24h">24h</option>
                    <option value="0">No TTL</option>
                  </select>
                </div>
                <p className="text-xs text-gray-500 mt-1">
                  Format: 30m, 1h, 24h, etc. Leave blank or set to 0 for no expiration.
                </p>
              </div>

              <div className="flex justify-end space-x-2">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-gray-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md"
                >
                  Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default ConsumerGroupsManager;
