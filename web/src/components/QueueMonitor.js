import React, { useState, useEffect, useRef } from 'react';
import { Loader, AlertTriangle, RefreshCw } from 'lucide-react';
import api from '../api';

const QueueMonitor = ({ domainName, queueName }) => {
  const [messages, setMessages] = useState([]);
  const [status, setStatus] = useState('disconnected'); // disconnected, connecting, connected, error
  const [error, setError] = useState(null);
  const webSocketRef = useRef(null);
  const messagesEndRef = useRef(null);
  const isClosingRef = useRef(false);  // Drapeau pour éviter les fermetures concurrentes
  const reconnectTimeoutRef = useRef(null); // Pour gérer les délais de reconnexion
  const reconnectAttemptRef = useRef(0); // Compteur de tentatives

  // Fonction pour se connecter au WebSocket
  const connectWebSocket = async () => {
    // Si déjà en train de se connecter, ne pas faire plusieurs tentatives
    if (status === 'connecting') return;
    
    // Nettoyer toute connexion existante
    await closeExistingConnection();

    // Mettre à jour l'état de l'interface
    setStatus('connecting');
    setError(null);

    try {
      // Essayer d'abord de s'abonner via l'API si disponible
      const subscription = await trySubscribeToQueue();
      
      // Créer et configurer la connexion WebSocket
      const monitor = createMonitorConnection();

      // Ajouter l'ID d'abonnement si disponible
      if (subscription?.subscriptionId) {
        monitor.subscriptionId = subscription.subscriptionId;
      }

      // Stocker la référence pour une utilisation ultérieure
      webSocketRef.current = monitor;

      // Configurer les gestionnaires d'événements WebSocket
      setupWebSocketHandlers(monitor);
      
      // Réinitialiser le compteur de tentatives en cas de succès
      reconnectAttemptRef.current = 0;
    } catch (err) {
      handleConnectionError(err);
    }
  };

  // Fermeture de connexion avec protection contre les opérations concurrentes
  const closeExistingConnection = async () => {
    // Éviter les fermetures concurrentes
    if (isClosingRef.current || !webSocketRef.current) return;
    
    isClosingRef.current = true;
    console.log('Fermeture connexion existante');
    
    try {
      const connection = webSocketRef.current;
      const subscriptionId = connection.subscriptionId;
      
      // Effacer la référence immédiatement pour éviter les accès concurrents
      webSocketRef.current = null;
      
      // Tenter de se désabonner d'abord avec un timeout
      if (subscriptionId) {
        try {
          // Utiliser un timeout pour éviter que le désabonnement ne bloque trop longtemps
          const unsubPromise = api.unsubscribeFromQueue(domainName, queueName, subscriptionId);
          const timeoutPromise = new Promise((_, reject) => 
            setTimeout(() => reject(new Error('Désabonnement timeout')), 2000)
          );
          
          await Promise.race([unsubPromise, timeoutPromise])
            .catch(err => console.warn(`Désabonnement ignoré: ${err.message}`));
        } catch (err) {
          console.warn('Erreur désabonnement ignorée:', err);
          // Ne pas bloquer le reste du nettoyage
        }
      }
      
      // Fermer la socket
      try {
        if (connection.socket && connection.socket.readyState < 2) {
          connection.socket.onclose = null; // Détacher l'événement pour éviter les rappels
          connection.socket.close();
        }
      } catch (err) {
        console.warn('Erreur fermeture socket ignorée:', err);
      }
      
      // Appeler la méthode close
      try {
        if (typeof connection.close === 'function') {
          connection.close();
        }
      } catch (err) {
        console.warn('Erreur fermeture connection ignorée:', err);
      }
    } finally {
      isClosingRef.current = false;
    }
  };

  const trySubscribeToQueue = async () => {
    if (typeof api.subscribeToQueue === 'function') {
      try {
        const sub = await api.subscribeToQueue(domainName, queueName);
        return sub;
      } catch (err) {
        console.warn('Subscription API not available:', err.message);
        return null;
      }
    }
    return null;
  };

  const createMonitorConnection = () => {
    return api.createQueueMonitor(
      domainName,
      queueName,
      handleIncomingMessage,
      handleWebSocketError
    );
  };

  const handleIncomingMessage = (message) => {
    console.log('Message reçu:', message);
    
    setMessages((prevMessages) => {
      // Adapter le format du message reçu
      const adaptedMessage = {
        id: message.ID || message.id || `anon-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        content: message.Payload || message.payload || message,
        headers: message.Headers || message.headers || {},
        receivedAt: new Date().toISOString()
      };
      
      // Éviter les doublons avec une meilleure détection
      if (prevMessages.some(m => m.id === adaptedMessage.id)) {
        return prevMessages;
      }
  
      const updatedMessages = [...prevMessages, adaptedMessage];
      if (updatedMessages.length > 100) {
        return updatedMessages.slice(-100);
      }
  
      return updatedMessages;
    });
  };

  const handleWebSocketError = (err) => {
    console.error('WebSocket error:', err);
    setStatus('error');
    
    // Vérifier si c'est une erreur fatale (comme domaine inexistant)
    const errorMsg = err.message || 'Unknown error';
    const isFatalError = errorMsg.includes('domain not found') || 
                        errorMsg.includes('queue not found');
    
    if (isFatalError) {
      setError(`Connection impossible: ${errorMsg}. Please verify domain and queue exist.`);
      // Ne pas réessayer pour les erreurs fatales
      reconnectAttemptRef.current = 999;
    } else {
      setError(`Connection error: ${errorMsg}. Will retry...`);
      scheduleReconnect();
    }
  };

  // Programmer une reconnexion avec backoff exponentiel
  const scheduleReconnect = () => {
    // Annuler toute reconnexion précédente
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }
    
    // Incrémenter le compteur de tentatives
    reconnectAttemptRef.current++;
    
    // Calculer le délai avec backoff exponentiel (1s, 2s, 4s, 8s...)
    // Mais pas plus de 30 secondes
    const maxAttempts = 10; // Maximum de tentatives
    if (reconnectAttemptRef.current <= maxAttempts) {
      const delay = Math.min(1000 * Math.pow(2, reconnectAttemptRef.current - 1), 30000);
      
      console.log(`Programmation reconnexion dans ${delay}ms (tentative ${reconnectAttemptRef.current})`);
      
      reconnectTimeoutRef.current = setTimeout(() => {
        reconnectTimeoutRef.current = null;
        connectWebSocket();
      }, delay);
    } else {
      console.log('Maximum de tentatives de reconnexion atteint');
    }
  };

  const setupWebSocketHandlers = (monitor) => {
    if (!monitor.socket) {
      setStatus('error');
      setError('WebSocket connection failed');
      return;
    }

    monitor.socket.onopen = () => {
      setStatus('connected');
      setError(null);
      // Réinitialiser le compteur de tentatives lors d'une connexion réussie
      reconnectAttemptRef.current = 0;
    };

    monitor.socket.onclose = (event) => {
      // Ne pas changer l'état si fermeture intentionnelle (isClosingRef.current === true)
      if (!isClosingRef.current) {
        console.log(`WebSocket closed: code=${event.code}, reason=${event.reason}, clean=${event.wasClean}`);
        
        if (event.wasClean) {
          // Fermeture propre
          setStatus('disconnected');
        } else {
          // Fermeture imprévue, tenter de se reconnecter
          setStatus('error');
          setError('Connection lost. Attempting to reconnect...');
          scheduleReconnect();
        }
      }
    };
  };

  const handleConnectionError = (err) => {
    console.error('Error creating WebSocket connection:', err);
    setStatus('error');
    
    // Vérifier si c'est une erreur fatale (comme domaine inexistant)
    const errorMsg = err.message || 'Unknown error';
    const isFatalError = errorMsg.includes('domain not found') || 
                        errorMsg.includes('queue not found');
    
    if (isFatalError) {
      setError(`Cannot connect: ${errorMsg}. Please verify domain and queue exist.`);
      // Ne pas réessayer pour les erreurs fatales
      reconnectAttemptRef.current = 999;
    } else {
      setError(`Failed to connect: ${errorMsg}. Will retry...`);
      scheduleReconnect();
    }
  };

  // Se connecter au WebSocket lors du montage du composant
  useEffect(() => {
    connectWebSocket();

    // Nettoyer à la déconnexion du composant
    return () => {
      // Annuler toute reconnexion programmée
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      
      closeExistingConnection();
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
            onClick={() => {
              // Réinitialiser le compteur pour une tentative manuelle
              reconnectAttemptRef.current = 0;
              connectWebSocket();
            }}
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
                <div key={message.id || index} className="p-3 hover:bg-gray-50">
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
