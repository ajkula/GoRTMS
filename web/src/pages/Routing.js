// web/src/pages/Routing.js
import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { PlusCircle, ChevronRight, Trash2, Loader, AlertTriangle } from 'lucide-react';
import api from '../api';

const Routing = () => {
  const { domainName } = useParams();
  const [rules, setRules] = useState([]);
  const [queues, setQueues] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  
  // État pour le formulaire de nouvelle règle
  const [newRule, setNewRule] = useState({
    sourceQueue: '',
    destinationQueue: '',
    predicate: {
      type: 'eq',
      field: '',
      value: ''
    }
  });
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState(null);

  // Charger les règles de routage et les files
  const fetchData = async () => {
    try {
      setLoading(true);
      setError(null);
      
      // Charger les règles de routage
      const routingRules = await api.getRoutingRules(domainName);
      setRules(routingRules);
      
      // Charger les files d'attente pour pouvoir les sélectionner
      const queueList = await api.getQueues(domainName);
      setQueues(queueList);
      
      // Si une file existe, préremplir le formulaire
      if (queueList.length > 0) {
        setNewRule(prev => ({
          ...prev,
          sourceQueue: queueList[0].name
        }));
      }
    } catch (err) {
      console.error('Error fetching routing data:', err);
      setError(err.message || 'Failed to load routing information');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [domainName]);

  // Ajouter une nouvelle règle de routage
  const handleAddRule = async (e) => {
    e.preventDefault();
    
    // Valider le formulaire
    if (!newRule.sourceQueue || !newRule.destinationQueue || 
        !newRule.predicate.field || !newRule.predicate.value) {
      setCreateError('All fields are required');
      return;
    }
    
    try {
      setCreateLoading(true);
      setCreateError(null);
      
      await api.addRoutingRule(domainName, newRule);
      
      // Réinitialiser le formulaire
      setNewRule({
        sourceQueue: queues.length > 0 ? queues[0].name : '',
        destinationQueue: '',
        predicate: {
          type: 'eq',
          field: '',
          value: ''
        }
      });
      
      // Recharger les règles
      await fetchData();
    } catch (err) {
      console.error('Error creating routing rule:', err);
      setCreateError(err.message || 'Failed to create routing rule');
    } finally {
      setCreateLoading(false);
    }
  };

  // Supprimer une règle de routage
  const handleDeleteRule = async (sourceQueue, destinationQueue) => {
    if (!window.confirm(`Are you sure you want to delete the routing rule from "${sourceQueue}" to "${destinationQueue}"?`)) {
      return;
    }
    
    try {
      await api.deleteRoutingRule(domainName, sourceQueue, destinationQueue);
      // Recharger les règles
      await fetchData();
    } catch (err) {
      console.error('Error deleting routing rule:', err);
      alert(`Failed to delete routing rule: ${err.message || 'Unknown error'}`);
    }
  };

  // Mettre à jour le prédicat dans le formulaire
  const handlePredicateChange = (field, value) => {
    setNewRule(prev => ({
      ...prev,
      predicate: {
        ...prev.predicate,
        [field]: value
      }
    }));
  };

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">
        <span className="text-gray-600">Domain:</span> {domainName}
      </h1>
      
      <h2 className="text-xl font-semibold mb-4">Routing Rules</h2>
      
      {/* Formulaire d'ajout de règle */}
      <div className="bg-white p-6 rounded-lg shadow-sm mb-6">
        <h3 className="text-lg font-medium mb-4">Create New Routing Rule</h3>
        
        <form onSubmit={handleAddRule}>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div>
              <label htmlFor="sourceQueue" className="block text-sm font-medium text-gray-700 mb-1">
                Source Queue
              </label>
              <select
                id="sourceQueue"
                value={newRule.sourceQueue}
                onChange={(e) => setNewRule(prev => ({ ...prev, sourceQueue: e.target.value }))}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                disabled={createLoading || queues.length === 0}
              >
                {queues.length === 0 ? (
                  <option>No queues available</option>
                ) : (
                  queues.map(queue => (
                    <option key={queue.name} value={queue.name}>{queue.name}</option>
                  ))
                )}
              </select>
            </div>
            
            <div>
              <label htmlFor="destinationQueue" className="block text-sm font-medium text-gray-700 mb-1">
                Destination Queue
              </label>
              <select
                id="destinationQueue"
                value={newRule.destinationQueue}
                onChange={(e) => setNewRule(prev => ({ ...prev, destinationQueue: e.target.value }))}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                disabled={createLoading || queues.length === 0}
              >
                <option value="">Select Destination Queue</option>
                {queues
                  .filter(queue => queue.name !== newRule.sourceQueue)
                  .map(queue => (
                    <option key={queue.name} value={queue.name}>{queue.name}</option>
                  ))
                }
              </select>
            </div>
          </div>
          
          <div className="bg-gray-50 p-4 rounded-md mb-4">
            <h4 className="text-sm font-medium text-gray-700 mb-3">Predicate (When to route)</h4>
            
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label htmlFor="predicateType" className="block text-sm font-medium text-gray-700 mb-1">
                  Condition Type
                </label>
                <select
                  id="predicateType"
                  value={newRule.predicate.type}
                  onChange={(e) => handlePredicateChange('type', e.target.value)}
                  className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                  disabled={createLoading}
                >
                  <option value="eq">Equals (==)</option>
                  <option value="neq">Not Equals (!=)</option>
                  <option value="gt">Greater Than (&gt;)</option>
                  <option value="gte">Greater Than or Equal (&gt;=)</option>
                  <option value="lt">Less Than (&lt;)</option>
                  <option value="lte">Less Than or Equal (&lt;=)</option>
                  <option value="contains">Contains</option>
                </select>
              </div>
              
              <div>
                <label htmlFor="predicateField" className="block text-sm font-medium text-gray-700 mb-1">
                  Field
                </label>
                <input
                  type="text"
                  id="predicateField"
                  value={newRule.predicate.field}
                  onChange={(e) => handlePredicateChange('field', e.target.value)}
                  placeholder="e.g. priority, type"
                  className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                  disabled={createLoading}
                />
              </div>
              
              <div>
                <label htmlFor="predicateValue" className="block text-sm font-medium text-gray-700 mb-1">
                  Value
                </label>
                <input
                  type="text"
                  id="predicateValue"
                  value={newRule.predicate.value}
                  onChange={(e) => handlePredicateChange('value', e.target.value)}
                  placeholder="e.g. high, alert"
                  className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                  disabled={createLoading}
                />
              </div>
            </div>
          </div>
          
          {createError && (
            <div className="mb-4 px-4 py-3 bg-red-50 text-red-700 text-sm rounded-md flex items-center">
              <AlertTriangle className="h-4 w-4 mr-1.5" />
              {createError}
            </div>
          )}
          
          <div className="flex justify-end">
            <button
              type="submit"
              className="inline-flex justify-center items-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
              disabled={!newRule.sourceQueue || !newRule.destinationQueue || !newRule.predicate.field || !newRule.predicate.value || createLoading}
            >
              {createLoading ? (
                <Loader className="h-5 w-5 animate-spin" />
              ) : (
                <>
                  <PlusCircle className="h-5 w-5 mr-1" />
                  Add Routing Rule
                </>
              )}
            </button>
          </div>
        </form>
      </div>
      
      {/* Liste des règles de routage */}
      <div className="bg-white rounded-lg shadow-sm overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h3 className="text-lg font-medium">Current Routing Rules</h3>
        </div>
        
        {loading ? (
          <div className="flex items-center justify-center h-32">
            <Loader className="h-6 w-6 animate-spin text-indigo-600" />
            <span className="ml-2">Loading routing rules...</span>
          </div>
        ) : error ? (
          <div className="p-6 text-center">
            <AlertTriangle className="h-8 w-8 text-red-500 mx-auto mb-2" />
            <h3 className="text-lg font-medium text-red-800">Failed to load routing rules</h3>
            <p className="text-sm text-red-600 mt-1">{error}</p>
            <button 
              onClick={fetchData}
              className="mt-3 bg-red-100 px-3 py-1 rounded-md text-red-800 hover:bg-red-200"
            >
              Retry
            </button>
          </div>
        ) : rules.length === 0 ? (
          <div className="p-6 text-center text-gray-500">
            No routing rules configured yet. Add your first rule above.
          </div>
        ) : (
          <ul className="divide-y divide-gray-200">
            {rules.map((rule, index) => (
              <li key={index} className="px-6 py-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center text-gray-900">
                    <span className="font-medium">{rule.sourceQueue}</span>
                    <ChevronRight className="h-5 w-5 mx-2 text-gray-400" />
                    <span className="font-medium">{rule.destinationQueue}</span>
                  </div>
                  
                  <button
                    onClick={() => handleDeleteRule(rule.sourceQueue, rule.destinationQueue)}
                    className="inline-flex items-center py-1 px-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  >
                    <Trash2 className="h-4 w-4 text-red-500" />
                  </button>
                </div>
                
                <div className="mt-2 text-sm text-gray-700 bg-gray-50 rounded px-3 py-2">
                  <div className="flex flex-wrap gap-1">
                    <span className="font-medium">When</span>
                    <span className="text-indigo-700">{rule.predicate.field}</span>
                    <span>
                      {rule.predicate.type === 'eq' && '='}
                      {rule.predicate.type === 'neq' && '!='}
                      {rule.predicate.type === 'gt' && '>'}
                      {rule.predicate.type === 'gte' && '>='}
                      {rule.predicate.type === 'lt' && '<'}
                      {rule.predicate.type === 'lte' && '<='}
                      {rule.predicate.type === 'contains' && 'contains'}
                    </span>
                    <span className="text-green-600">"{rule.predicate.value}"</span>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
};

export default Routing;