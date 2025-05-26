import React, { useState, useEffect } from 'react';
import { PlusCircle, ChevronRight, Trash2, Loader, AlertTriangle, Columns } from 'lucide-react';
import api from '../api';
import RoutingTester from '../components/RoutingTester';

const Routing = () => {
  const [domains, setDomains] = useState([]);
  const [domainName, setDomainName] = useState('');
  const [rules, setRules] = useState([]);
  const [queues, setQueues] = useState([]);
  const [loading, setLoading] = useState(true);
  const [selectedSourceQueue, setSelectedSourceQueue] = useState('');
  const [error, setError] = useState(null);

  const fetchDomains = async () => {
    console.log('Fetching domains...');
    try {
      setLoading(true);
      setError(null);
      const domainsData = await api.getDomains();

      const detailedDomains = await Promise.all(
        domainsData.map(async (domain) => {
          try {
            const details = await api.getDomainDetails(domain.name);
            return {
              ...domain,
              queueCount: details.queues ? details.queues.length : domain.queueCount || 0,
              messageCount: details.queues
                ? details.queues.reduce((total, q) => total + (q.messageCount || 0), 0)
                : domain.messageCount || 0
            };
          } catch (err) {
            console.log(`Couldn't fetch details for domain ${domain.name}`, err);
            return domain;
          }
        })
      );

      setDomains(detailedDomains);
      if (detailedDomains.length > 0) {
        setDomainName(detailedDomains[0].name);
      } else {
        console.log("No domains available");
        setDomainName('');
      }
    } catch (err) {
      console.error('Error fetching domains:', err);
      setError(err.message || 'Failed to load domains');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDomains();
  }, []);

  // State for the new rule form
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

  // Load routing rules and queues
  const fetchData = async () => {
    try {
      setLoading(true);
      setError(null);

      const routingRules = await api.getRoutingRules(domainName);
      setRules(routingRules);

      const queueList = await api.getQueues(domainName);
      setQueues(queueList);

      // If a queue exists, pre-fill the form
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
    if (queues.length > 0 && !selectedSourceQueue) {
      setSelectedSourceQueue(queues[0].name);
    }
  }, [queues, selectedSourceQueue]);

  useEffect(() => {
    if (domainName) {
      fetchData();
    }
  }, [domainName]);

  // Add a new routing rule
  const handleAddRule = async (e) => {
    e.preventDefault();

    // Validate form
    if (!newRule.sourceQueue || !newRule.destinationQueue ||
      !newRule.predicate.field || !newRule.predicate.value) {
      setCreateError('All fields are required');
      return;
    }

    try {
      setCreateLoading(true);
      setCreateError(null);

      await api.addRoutingRule(domainName, newRule);

      // reset form
      setNewRule({
        sourceQueue: queues.length > 0 ? queues[0].name : '',
        destinationQueue: '',
        predicate: {
          type: 'eq',
          field: '',
          value: ''
        }
      });

      await fetchData();
    } catch (err) {
      console.error('Error creating routing rule:', err);
      setCreateError(err.message || 'Failed to create routing rule');
    } finally {
      setCreateLoading(false);
    }
  };

  const handleDeleteRule = async (sourceQueue, destinationQueue) => {
    if (!window.confirm(`Are you sure you want to delete the routing rule from "${sourceQueue}" to "${destinationQueue}"?`)) {
      return;
    }

    try {
      await api.deleteRoutingRule(domainName, sourceQueue, destinationQueue);
      await fetchData();
    } catch (err) {
      console.error('Error deleting routing rule:', err);
      alert(`Failed to delete routing rule: ${err.message || 'Unknown error'}`);
    }
  };

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

      {/* Rule addition form */}
      <div className="bg-white p-6 rounded-lg shadow-sm mb-6">
        <h3 className="text-lg font-medium mb-4">Create New Routing Rule</h3>

        <form onSubmit={handleAddRule}>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div>
              <label htmlFor="sourceQueue" className="block text-sm font-medium text-gray-700 mb-1">
                Select domain
              </label>
              <select
                id="domainName"
                value={domainName}
                onChange={(e) => { setDomainName(e.target.value); console.log(e.target.value) }}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                disabled={createLoading}
              >
                {domains.length === 0 ? (
                  <option>No domain available</option>
                ) : (
                  domains.map(dom => (
                    <option key={dom.name} value={dom.name}>{dom.name}</option>
                  ))
                )}
              </select>
            </div>
          </div>
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

      {/* Routing rules list */}
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
                    <span className="font-medium">{rule.SourceQueue}</span>
                    <ChevronRight className="h-5 w-5 mx-2 text-gray-400" />
                    <span className="font-medium">{rule.DestinationQueue}</span>
                  </div>

                  <button
                    onClick={() => handleDeleteRule(rule.SourceQueue, rule.DestinationQueue)}
                    className="inline-flex items-center py-1 px-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  >
                    <Trash2 className="h-4 w-4 text-red-500" />
                  </button>
                </div>

                <div className="mt-2 text-sm text-gray-700 bg-gray-50 rounded px-3 py-2">
                  {(() => {
                    const pred = rule.Predicate;

                    return (
                      <div className="flex flex-wrap gap-1">
                        <span className="font-medium">When</span>
                        <span className="text-indigo-700">{pred && pred.field}</span>
                        <span>
                          {pred.type === 'eq' && '='}
                          {pred.type === 'neq' && '!='}
                          {pred.type === 'gt' && '>'}
                          {pred.type === 'gte' && '>='}
                          {pred.type === 'lt' && '<'}
                          {pred.type === 'lte' && '<='}
                          {pred.type === 'contains' && 'contains'}
                        </span>
                        <span className="text-green-600">"{pred.value}"</span>
                      </div>
                    );
                  })()}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
      {newRule.sourceQueue && (
        <RoutingTester
          domainName={domainName}
          sourceQueue={selectedSourceQueue}
          rules={rules.filter(rule => rule.SourceQueue === selectedSourceQueue)}
        />
      )}
    </div>
  );
};

export default Routing;