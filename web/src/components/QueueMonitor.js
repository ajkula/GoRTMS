// web/src/components/queue/QueueMonitor.js
import React, { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { Loader, AlertTriangle, RefreshCw } from 'lucide-react';
import api from '../api';

const QueueMonitor = () => {
  const { domainName, queueName } = useParams();
  const [messages, setMessages] = useState([]);
  const [status, setStatus] = useState('disconnected'); // disconnected, connecting, connected, error
  const [error, setError] = useState(null);
  const webSocketRef = useRef(null);
  const messagesEndRef = useRef(null);

  // Fonction pour se connecter au WebSocket
  const connectWebSocket = () => {
    // Fermer la connexion existante si elle existe
    if (webSocketRef.current) {
      webSocketRef.current.close();
      webSocketRef.current = null;
    }

    setStatus('connecting');
    setError(null);

    try {
      // Abonner d'abord via l'API REST
      api.subscribeToQueue(domainName, queueName)
        .then(() => {
          // Puis créer la connexion WebSocket
          const monitor = api.createQueueMonitor(
            domainName,
            queueName,
            // Gestionnaire de messages
            (message) => {
              setMessages((prevMessages) => {
                // Limiter à 100 messages pour éviter les problèmes de performance
                const newMessages = [...prevMessages, {
                  ...message,
                  receivedAt: new Date().toISOString()
                }];
                if (newMessages.length > 100) {
                  newMessages.shift(); // Supprimer le message le plus ancien
                }
                return newMessages;
              });
            },
            // Gestionnaire d'erreurs
            (err) => {
              console.error('WebSocket error:', err);
              setStatus('error');
              setError('Connection error. Try reconnecting.');
            }
          );

          webSocketRef.current = monitor;

          // Ajouter des gestionnaires d'événements supplémentaires
          monitor.socket.onopen = () => {
            setStatus('connected');
            setError(null);
          };

          monitor.socket.onclose = () => {
            setStatus('disconnected');
          };
        })
    } catch (err) {
      console.error('Error creating WebSocket connection:', err);
      setStatus('error');
      setError(err.message || 'Failed to connect to queue monitor');
    }
  };

  // Se connecter au WebSocket lors du montage du composant
  useEffect(() => {
    connectWebSocket();

    // Nettoyer à la déconnexion
    return () => {
      if (webSocketRef.current) {
        webSocketRef.current.close();
        // Tenter de se désabonner si nous avons un ID d'abonnement
        if (webSocketRef.current.subscriptionId) {
          api.unsubscribeFromQueue(domainName, queueName, webSocketRef.current.subscriptionId)
            .catch(err => console.error('Error unsubscribing:', err));
        }
      }
    };
  }, [domainName, queueName]);

  // Faire défiler vers le dernier message quand de nouveaux messages arrivent
  useEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages]);

  // Formater le contenu du message pour l'affichage
  const formatMessageContent = (content) => {
    try {
      if (typeof content === 'string') {
        // Essayer de parser comme JSON pour un affichage formaté
        const parsed = JSON.parse(content);
        return <pre className="overflow-auto text-xs">{JSON.stringify(parsed, null, 2)}</pre>;
      } else if (typeof content === 'object') {
        return <pre className="overflow-auto text-xs">{JSON.stringify(content, null, 2)}</pre>;
      }
      return String(content);
    } catch (e) {
      // Si ce n'est pas du JSON valide, afficher comme texte brut
      return <span className="break-words">{String(content)}</span>;
    }
  };

  return (
    <div className="bg-white rounded-lg shadow-sm overflow-hidden">
      <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center">
        <h2 className="text-lg font-medium">
          Queue Monitor: <span className="font-semibold">{queueName}</span>
        </h2>

        <div className="flex items-center space-x-2">
          {/* Indicateur de statut */}
          <div className="flex items-center">
            {status === 'connected' && (
              <span className="flex items-center text-green-600 text-sm">
                <span className="h-2 w-2 rounded-full bg-green-500 mr-1.5"></span>
                Connected
              </span>
            )}
            {status === 'connecting' && (
              <span className="flex items-center text-yellow-600 text-sm">
                <Loader className="h-3 w-3 animate-spin mr-1.5" />
                Connecting...
              </span>
            )}
            {(status === 'disconnected' || status === 'error') && (
              <span className="flex items-center text-red-600 text-sm">
                <span className="h-2 w-2 rounded-full bg-red-500 mr-1.5"></span>
                {status === 'error' ? 'Error' : 'Disconnected'}
              </span>
            )}
          </div>

          {/* Bouton de reconnexion */}
          <button
            onClick={connectWebSocket}
            className="inline-flex items-center py-1 px-2 text-xs border border-gray-300 rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-indigo-500"
            disabled={status === 'connecting'}
          >
            <RefreshCw className="h-3 w-3 mr-1" />
            Reconnect
          </button>
        </div>
      </div>

      {error && (
        <div className="px-6 py-3 bg-red-50 text-red-700 text-sm flex items-center">
          <AlertTriangle className="h-4 w-4 mr-1.5" />
          {error}
        </div>
      )}

      <div className="p-4">
        <div className="border border-gray-200 rounded-md h-96 overflow-y-auto">
          {messages.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
              <p>No messages received yet.</p>
              <p className="text-sm mt-1">
                {status === 'connected'
                  ? 'Waiting for new messages...'
                  : 'Connect to start monitoring messages.'}
              </p>
            </div>
          ) : (
            <div className="divide-y divide-gray-100">
              {messages.map((message, index) => (
                <div key={index} className="p-3 hover:bg-gray-50">
                  <div className="flex justify-between text-xs text-gray-500 mb-1">
                    <span>ID: {message.id || 'N/A'}</span>
                    <span>
                      {new Date(message.receivedAt).toLocaleTimeString()}
                    </span>
                  </div>
                  <div className="mt-1">
                    {formatMessageContent(message.content)}
                  </div>
                  {message.headers && Object.keys(message.headers).length > 0 && (
                    <div className="mt-2 pt-1 border-t border-gray-100">
                      <details className="text-xs">
                        <summary className="cursor-pointer text-gray-500 hover:text-gray-700">
                          Headers
                        </summary>
                        <div className="mt-1 pl-2">
                          <pre className="text-xs">{JSON.stringify(message.headers, null, 2)}</pre>
                        </div>
                      </details>
                    </div>
                  )}
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default QueueMonitor;