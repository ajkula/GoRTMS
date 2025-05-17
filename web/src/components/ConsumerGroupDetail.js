import React, { useState, useEffect } from 'react';
import { ArrowLeft, RefreshCw, Clock, Trash2, MessageCircle, User, Save, AlertTriangle } from 'lucide-react';
import api from '../api';
import { formatDuration } from '../utils/utils';

const ConsumerGroupDetail = ({ domainName, queueName, groupID, onBack }) => {
  const [group, setGroup] = useState(null);
  const [pendingMessages, setPendingMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [ttl, setTTL] = useState('');
  const [ttlInput, setTTLInput] = useState('');
  const [showTTLEditor, setShowTTLEditor] = useState(false);

  const isDefaultDate = (dateStr) => {
    return !dateStr || dateStr === "0001-01-01T00:00:00Z";
  }

  // Charger les détails du consumer group
  const fetchGroupDetails = async () => {
    try {
      setLoading(true);
      setError(null);

      console.log(`Fetching group details for ${domainName}/${queueName}/${groupID}`);
      const groupData = await api.getConsumerGroup(domainName, queueName, groupID);
      console.log("Received group data:", groupData);

      // Normaliser les données pour gérer les différentes casses
      const normalizedGroup = {
        ...groupData,
        // Assurez-vous que ces propriétés existent quelle que soit la casse
        consumerIDs: groupData.ConsumerIDs || groupData.consumerIDs || [],
        lastActivity: groupData.LastActivity || groupData.lastActivity,
        position: groupData.Position || groupData.position || 0
      };

      setGroup(normalizedGroup);
      setTTL(groupData.ttl || '0');
      setTTLInput(groupData.ttl || '0');

      // Charger les messages en attente pour ce groupe
      console.log(`Fetching pending messages for ${domainName}/${queueName}/${groupID}`);
      const messages = await api.getPendingMessages(domainName, queueName, groupID);
      console.log("Received pending messages:", messages);

      // S'assurer que messages est un tableau
      setPendingMessages(Array.isArray(messages) ? messages : (messages.messages || []));
    } catch (err) {
      console.error('Error fetching consumer group details:', err);
      setError(err.message || 'Failed to load consumer group details');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchGroupDetails();

    // Rafraîchir périodiquement
    const interval = setInterval(fetchGroupDetails, 10000);
    return () => clearInterval(interval);
  }, [domainName, queueName, groupID]);

  // Mettre à jour le TTL avec gestion d'erreur améliorée
  const handleUpdateTTL = async () => {
    try {
      setLoading(true);
      console.log(`Updating TTL for ${domainName}/${queueName}/${groupID} to ${ttlInput}`);
      await api.updateConsumerGroupTTL(domainName, queueName, groupID, ttlInput);
      setTTL(ttlInput);
      setShowTTLEditor(false);
      await fetchGroupDetails();
    } catch (err) {
      console.error('Error updating TTL:', err);
      setError(err.message || 'Failed to update TTL');
    } finally {
      setLoading(false);
    }
  };

  // Accès sécurisé aux propriétés du groupe
  const validConsumerIds = group?.consumerIDs?.filter(id => id !== '') || [];

  // Supprimer un consumer du groupe
  const handleRemoveConsumer = async (consumerID) => {
    if (!window.confirm(`Remove consumer "${consumerID}" from group?`)) {
      return;
    }

    try {
      setLoading(true);
      await api.removeConsumerFromGroup(domainName, queueName, groupID, consumerID);
      await fetchGroupDetails();
    } catch (err) {
      console.error('Error removing consumer:', err);
      setError(err.message || 'Failed to remove consumer');
    } finally {
      setLoading(false);
    }
  };

  const handleRecreateGroup = async () => {
    try {
      setLoading(true);
      setError(null);

      // Essayer de supprimer le groupe existant (peut-être bloqué)
      try {
        await api.deleteConsumerGroup(domainName, queueName, groupID);
      } catch (err) {
        console.log("Groupe non supprimé ou déjà inexistant, on continue:", err);
      }

      // Recréer le groupe avec les mêmes paramètres
      await api.createConsumerGroup(domainName, queueName, {
        groupID: groupID,
        ttl: ttl || "1h" // Utiliser le TTL actuel ou 1h par défaut
      });

      // Rafraîchir les données
      await fetchGroupDetails();

    } catch (err) {
      console.error("Erreur lors de la recréation du groupe:", err);
      setError("Impossible de recréer le groupe: " + err.message);
    } finally {
      setLoading(false);
    }
  };

  if (loading && !group) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-indigo-600" />
        <span className="ml-2">Loading consumer group details...</span>
      </div>
    );
  }

  return (
    <div className="container mx-auto">
      <div className="flex items-center mb-6">
        <button
          onClick={onBack}
          className="mr-3 p-2 rounded-full text-gray-500 hover:bg-gray-100"
        >
          <ArrowLeft className="h-5 w-5" />
        </button>
        <div>
          <h1 className="text-2xl font-bold">
            Consumer Group: <span className="text-indigo-600">{groupID}</span>
          </h1>
          <p className="text-gray-600">
            Domain: {domainName} / Queue: {queueName}
          </p>
        </div>
        <button
          onClick={fetchGroupDetails}
          className="ml-auto inline-flex items-center px-3 py-2 border border-gray-300 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
          disabled={loading}
        >
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
        <button
          onClick={handleRecreateGroup}
          className="ml-2 inline-flex items-center px-3 py-2 border border-red-300 shadow-sm text-sm leading-4 font-medium rounded-md text-red-700 bg-white hover:bg-red-50"
          disabled={loading}
        >
          <RefreshCw className="h-4 w-4 mr-1" />
          Reset Group
        </button>
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

      {/* Information générale */}
      <div className="bg-white shadow overflow-hidden sm:rounded-lg mb-6">
        <div className="px-4 py-5 sm:px-6 flex justify-between items-center">
          <h3 className="text-lg leading-6 font-medium text-gray-900">
            Group Information
          </h3>
        </div>
        <div className="border-t border-gray-200">
          <dl>
            <div className="bg-gray-50 px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
              <dt className="text-sm font-medium text-gray-500">Group ID</dt>
              <dd className="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">{groupID}</dd>
            </div>
            <div className="bg-white px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
              <dt className="text-sm font-medium text-gray-500">Last Activity</dt>
              <dd className="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                {group && group.lastActivity ? new Date(group.lastActivity).toLocaleString() : 'Never'}
              </dd>
            </div>
            <div className="bg-gray-50 px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
              <dt className="text-sm font-medium text-gray-500">Last Message Position</dt>
              <dd className="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                {group && group.position ? group.position : 'None'}
              </dd>
            </div>
            <div className="bg-white px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
              <dt className="text-sm font-medium text-gray-500">TTL (Time to Live)</dt>
              <dd className="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2 flex items-center">
                {showTTLEditor ? (
                  <div className="flex space-x-2 items-center">
                    <input
                      type="text"
                      value={ttlInput}
                      onChange={(e) => setTTLInput(e.target.value)}
                      className="border border-gray-300 rounded-md p-1 text-sm"
                      placeholder="e.g. 30m, 1h, 24h"
                    />
                    <select
                      onChange={(e) => setTTLInput(e.target.value)}
                      className="border border-gray-300 rounded-md p-1 text-sm"
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
                    <button
                      onClick={handleUpdateTTL}
                      className="p-1 text-indigo-600 hover:text-indigo-800"
                      disabled={loading}
                    >
                      <Save className="h-4 w-4" />
                    </button>
                    <button
                      onClick={() => {
                        setTTLInput(ttl);
                        setShowTTLEditor(false);
                      }}
                      className="p-1 text-gray-600 hover:text-gray-800"
                    >
                      Cancel
                    </button>
                  </div>
                ) : (
                  <>
                    <span className="mr-2">{formatDuration(ttl)}</span>
                    <button
                      onClick={() => setShowTTLEditor(true)}
                      className="p-1 text-indigo-600 hover:text-indigo-800"
                    >
                      <Clock className="h-4 w-4" />
                    </button>
                  </>
                )}
              </dd>
            </div>
          </dl>
        </div>
      </div>

      {/* Liste des consommateurs */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-white shadow sm:rounded-lg">
          <div className="px-4 py-5 sm:px-6">
            <h3 className="text-lg leading-6 font-medium text-gray-900 flex items-center">
              <User className="h-5 w-5 mr-2" />
              Active Consumers
            </h3>
            <p className="mt-1 max-w-2xl text-sm text-gray-500">
              List of consumers currently associated with this group.
            </p>
          </div>
          <div className="border-t border-gray-200">
            {!group || validConsumerIds.length === 0 ? (
              <div className="px-4 py-5 text-center text-gray-500">
                No active consumers in this group.
              </div>
            ) : (
              <ul className="divide-y divide-gray-200">
                {validConsumerIds.map((consumerId) => (
                  <li key={consumerId} className="px-4 py-4 flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-gray-900">{consumerId}</p>
                      <p className="text-sm text-gray-500">
                        Last activity: {isDefaultDate(group.lastActivity)
                          ? 'Never'
                          : new Date(group.lastActivity).toLocaleString()}
                      </p>
                    </div>
                    <button
                      onClick={() => handleRemoveConsumer(consumerId)}
                      className="p-2 text-red-600 hover:text-red-800"
                      disabled={loading}
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        {/* Messages en attente */}
        <div className="bg-white shadow sm:rounded-lg">
          <div className="px-4 py-5 sm:px-6">
            <h3 className="text-lg leading-6 font-medium text-gray-900 flex items-center">
              <MessageCircle className="h-5 w-5 mr-2" />
              Pending Messages
            </h3>
            <p className="mt-1 max-w-2xl text-sm text-gray-500">
              Messages waiting for acknowledgment by this group.
            </p>
          </div>
          <div className="border-t border-gray-200">
            {!pendingMessages || pendingMessages.length === 0 ? (
              <div className="px-4 py-5 text-center text-gray-500">
                No pending messages for this group.
              </div>
            ) : (
              <ul className="divide-y divide-gray-200 max-h-72 overflow-y-auto">
                {pendingMessages.map((message) => (
                  <li key={message.id} className="px-4 py-4 flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-gray-900">ID: {message.id}</p>
                      <p className="text-sm text-gray-500">
                        Timestamp: {new Date(message.timestamp).toLocaleString()}
                      </p>
                      <div className="mt-1 text-xs text-gray-500 max-w-md truncate">
                        Payload: {JSON.stringify(message.payload || message.content)}
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default ConsumerGroupDetail;
