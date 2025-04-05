import React, { useState, useEffect } from 'react';
import { PlusCircle, Trash2, Loader, AlertTriangle, Eye, ArrowLeft } from 'lucide-react';
import api from '../api';

const QueuesManager = ({ domainName, onBack, onSelectQueue }) => {
  const [queues, setQueues] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [newQueueName, setNewQueueName] = useState('');
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState(null);

  // Charger les files d'attente pour le domaine spécifié
  const fetchQueues = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.getQueues(domainName);
      setQueues(data);
    } catch (err) {
      console.error(`Error fetching queues for domain ${domainName}:`, err);
      setError(err.message || 'Failed to load queues');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchQueues();
  }, [domainName]);

  // Créer une nouvelle file d'attente
  const handleCreateQueue = async (e) => {
    e.preventDefault();
    if (!newQueueName.trim()) return;
    
    try {
      setCreateLoading(true);
      setCreateError(null);
      
      await api.createQueue(domainName, {
        name: newQueueName,
        config: {
          isPersistent: true,
          maxSize: 1000,
          ttl: "86400s", // 24 heures
          deliveryMode: "broadcast"
        }
      });
      
      setNewQueueName('');
      await fetchQueues();
    } catch (err) {
      console.error('Error creating queue:', err);
      setCreateError(err.message || 'Failed to create queue');
    } finally {
      setCreateLoading(false);
    }
  };

  // Supprimer une file d'attente
  const handleDeleteQueue = async (queueName) => {
    if (!window.confirm(`Are you sure you want to delete queue "${queueName}"? This will also delete all its messages.`)) {
      return;
    }
    
    try {
      await api.deleteQueue(domainName, queueName);
      await fetchQueues();
    } catch (err) {
      console.error(`Error deleting queue ${queueName}:`, err);
      alert(`Failed to delete queue: ${err.message || 'Unknown error'}`);
    }
  };

  return (
    <div>
      <div className="flex items-center mb-6">
        <button 
          onClick={onBack}
          className="mr-3 inline-flex items-center justify-center p-2 rounded-md text-gray-500 hover:bg-gray-100"
        >
          <ArrowLeft className="h-5 w-5" />
        </button>
        <h1 className="text-2xl font-bold">
          Queues: <span className="text-indigo-600">{domainName}</span>
        </h1>
      </div>
      
      {/* Formulaire de création de file d'attente */}
      <div className="bg-white p-6 rounded-lg shadow-sm mb-6">
        <h2 className="text-lg font-medium mb-4">Create New Queue</h2>
        
        <form onSubmit={handleCreateQueue} className="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-2">
          <input
            type="text"
            value={newQueueName}
            onChange={(e) => setNewQueueName(e.target.value)}
            placeholder="Queue name"
            className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50 p-2"
            disabled={createLoading}
          />
          
          <button
            type="submit"
            className="inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            disabled={!newQueueName.trim() || createLoading}
          >
            {createLoading ? (
              <Loader className="h-5 w-5 animate-spin" />
            ) : (
              <>
                <PlusCircle className="h-5 w-5 mr-1" />
                Create Queue
              </>
            )}
          </button>
        </form>
        
        {createError && (
          <div className="mt-2 text-sm text-red-600">
            <AlertTriangle className="h-4 w-4 inline mr-1" />
            {createError}
          </div>
        )}
      </div>
      
      {/* Liste des files d'attente */}
      <div className="bg-white rounded-lg shadow-sm overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium">Queue List</h2>
        </div>
        
        {loading && queues.length === 0 ? (
          <div className="flex items-center justify-center h-32">
            <Loader className="h-6 w-6 animate-spin text-indigo-600" />
            <span className="ml-2">Loading queues...</span>
          </div>
        ) : error ? (
          <div className="p-6 text-center">
            <AlertTriangle className="h-8 w-8 text-red-500 mx-auto mb-2" />
            <h3 className="text-lg font-medium text-red-800">Failed to load queues</h3>
            <p className="text-sm text-red-600 mt-1">{error}</p>
            <button 
              onClick={fetchQueues}
              className="mt-3 bg-red-100 px-3 py-1 rounded-md text-red-800 hover:bg-red-200"
            >
              Retry
            </button>
          </div>
        ) : queues.length === 0 ? (
          <div className="p-6 text-center text-gray-500">
            No queues available. Create your first queue above.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Queue Name
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Messages
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Config
                  </th>
                  <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {queues.map((queue) => (
                  <tr key={queue.name} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="font-medium text-gray-900">{queue.name}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-gray-500">{queue.messageCount || 0}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-xs text-gray-500">
                        {queue.config && (
                          <span className="space-x-2">
                            <span className={`inline-flex px-2 py-1 rounded-full ${queue.config.isPersistent ? 'bg-green-100 text-green-800' : 'bg-yellow-100 text-yellow-800'}`}>
                              {queue.config.isPersistent ? 'Persistent' : 'Temporary'}
                            </span>
                            <span className="inline-flex px-2 py-1 rounded-full bg-blue-100 text-blue-800">
                              {queue.config.deliveryMode || 'Broadcast'}
                            </span>
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button
                        onClick={() => onSelectQueue(queue.name)}
                        className="text-indigo-600 hover:text-indigo-900 mr-3"
                      >
                        <Eye className="h-4 w-4 inline mr-1" />
                        Monitor
                      </button>
                      <button
                        onClick={() => handleDeleteQueue(queue.name)}
                        className="text-red-600 hover:text-red-900"
                      >
                        <Trash2 className="h-4 w-4 inline" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};

export default QueuesManager;