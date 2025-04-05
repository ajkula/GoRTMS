import React, { useState, useEffect } from 'react';
import { PlusCircle, Trash2, Loader, AlertTriangle, ExternalLink } from 'lucide-react';
import api from '../api';

const DomainsManager = ({ onSelectDomain }) => {
  const [domains, setDomains] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [newDomainName, setNewDomainName] = useState('');
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState(null);

  // Charger les domaines
  const fetchDomains = async () => {
    console.log('Fetching domains...');
    try {
      setLoading(true);
      setError(null);
      const domainsData = await api.getDomains();
      console.log('Domains received:', domainsData);
      
      // Si nous avons besoin de plus de détails pour chaque domaine
      const detailedDomains = await Promise.all(
        domainsData.map(async (domain) => {
          try {
            // Essayer de récupérer les détails du domaine si l'API le permet
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
            return domain; // Conserver le domaine tel quel si pas de détails
          }
        })
      );
      
      setDomains(detailedDomains);
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

  // Créer un nouveau domaine
  const handleCreateDomain = async (e) => {
    e.preventDefault();
    if (!newDomainName.trim()) return;
    
    try {
      setCreateLoading(true);
      setCreateError(null);
      
      const creation = await api.createDomain({ 
        name: newDomainName,
        schema: {
          fields: {
            type: "string",
            content: "string"
          }
        }
      });
      console.log(creation)
      
      setNewDomainName('');
      await fetchDomains();
    } catch (err) {
      console.error('Error creating domain:', err);
      setCreateError(err.message || 'Failed to create domain');
    } finally {
      setCreateLoading(false);
    }
  };

  // Supprimer un domaine
  const handleDeleteDomain = async (domainName) => {
    if (!window.confirm(`Are you sure you want to delete domain "${domainName}"? This will also delete all its queues and messages.`)) {
      return;
    }
    
    try {
      await api.deleteDomain(domainName);
      await fetchDomains();
    } catch (err) {
      console.error(`Error deleting domain ${domainName}:`, err);
      alert(`Failed to delete domain: ${err.message || 'Unknown error'}`);
    }
  };

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Domains</h1>
      
      {/* Formulaire de création de domaine */}
      <div className="bg-white p-6 rounded-lg shadow-sm mb-6">
        <h2 className="text-lg font-medium mb-4">Create New Domain</h2>
        
        <form onSubmit={handleCreateDomain} className="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-2">
          <input
            type="text"
            value={newDomainName}
            onChange={(e) => setNewDomainName(e.target.value)}
            placeholder="Domain name"
            className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50 p-2"
            disabled={createLoading}
          />
          
          <button
            type="submit"
            className="inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            disabled={!newDomainName.trim() || createLoading}
          >
            {createLoading ? (
              <Loader className="h-5 w-5 animate-spin" />
            ) : (
              <>
                <PlusCircle className="h-5 w-5 mr-1" />
                Create Domain
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
      
      {/* Liste des domaines */}
      <div className="bg-white rounded-lg shadow-sm overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium">Domain List</h2>
        </div>
        
        {loading && domains.length === 0 ? (
          <div className="flex items-center justify-center h-32">
            <Loader className="h-6 w-6 animate-spin text-indigo-600" />
            <span className="ml-2">Loading domains...</span>
          </div>
        ) : error ? (
          <div className="p-6 text-center">
            <AlertTriangle className="h-8 w-8 text-red-500 mx-auto mb-2" />
            <h3 className="text-lg font-medium text-red-800">Failed to load domains</h3>
            <p className="text-sm text-red-600 mt-1">{error}</p>
            <button 
              onClick={fetchDomains}
              className="mt-3 bg-red-100 px-3 py-1 rounded-md text-red-800 hover:bg-red-200"
            >
              Retry
            </button>
          </div>
        ) : domains.length === 0 ? (
          <div className="p-6 text-center text-gray-500">
            No domains available. Create your first domain above.
          </div>
        ) : (
          <ul className="divide-y divide-gray-200">
            {domains.map((domain) => (
              <li key={domain.name} className="px-6 py-4 flex items-center justify-between hover:bg-gray-50">
                <div>
                  <h3 className="text-lg font-medium text-gray-900">{domain.name}</h3>
                  <p className="text-sm text-gray-500">
                    {domain.queueCount || 0} queues · {domain.messageCount || 0} messages
                  </p>
                </div>
                
                <div className="flex space-x-2">
                  <button
                    onClick={() => onSelectDomain(domain.name)}
                    className="inline-flex items-center justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-indigo-600 bg-indigo-100 hover:bg-indigo-200 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                  >
                    <ExternalLink className="h-4 w-4 mr-1" />
                    Manage
                  </button>
                  
                  <button
                    onClick={() => handleDeleteDomain(domain.name)}
                    className="inline-flex items-center py-2 px-3 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                  >
                    <Trash2 className="h-4 w-4 text-red-500" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
};

export default DomainsManager;