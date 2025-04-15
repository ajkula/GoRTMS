const API_BASE_URL = '/api';

const api = {
  // Fonction utilitaire pour gérer les erreurs et parser les réponses JSON
  async fetchJSON(url, options = {}) {
    try {
      const response = await fetch(url, options);
      if (!response.ok) {
        throw new Error(`API error: ${response.status} ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error(`API fetch error for ${url}: ${error.message}`);
      throw error;
    }
  },

  // Domaines
  async getDomains() {
    try {
      const data = await this.fetchJSON(`${API_BASE_URL}/domains`);
      return data.domains || [];
    } catch (error) {
      console.error('Error fetching domains:', error);
      return []; // Retourner un tableau vide en cas d'erreur
    }
  },

  async createDomain(domain) {
    return this.fetchJSON(`${API_BASE_URL}/domains`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(domain)
    });
  },

  async getDomainDetails(domainName) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}`);
  },

  async deleteDomain(domainName) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}`, {
      method: 'DELETE'
    });
  },

  // Files d'attente
  async getQueues(domainName) {
    try {
      const data = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues`);
      return data.queues || [];
    } catch (error) {
      console.error(`Error fetching queues for domain ${domainName}:`, error);
      return []; // Retourner un tableau vide en cas d'erreur
    }
  },

  async createQueue(domainName, queue) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(queue)
    });
  },

  async deleteQueue(domainName, queueName) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}`, {
      method: 'DELETE'
    });
  },

  async subscribeToQueue(domainName, queueName) {
    
  
  console.log({
    domainName,
    queueName
  });


    try {
      // Cette API est juste pour l'enregistrement initial
      const response = await fetch(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/subscribe`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({}), // Corps vide ou avec callbackUrl si nécessaire
      });

      if (!response.ok) {
        throw new Error(`Error subscribing to queue: ${response.statusText}`);
      }
      const data =  await response.json();
      console.log({ data });

      return data;
    } catch (error) {
      console.error(`Error subscribing to queue ${domainName}/${queueName}:`, error);
      throw error;
    }
  },

  async unsubscribeFromQueue(domainName, queueName, subscriptionId) {
    try {
      const response = await fetch(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/unsubscribe`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ subscriptionId }), // Passer l'ID d'abonnement dans le corps
      });

      if (!response.ok) {
        throw new Error(`Error unsubscribing from queue: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error(`Error unsubscribing from queue ${domainName}/${queueName}:`, error);
      throw error;
    }
  },

  // Messages
  async publishMessage(domainName, queueName, message) {
    // Si le message a content et headers, extraire content pour le payload principal
    let payload = message;
    let headers = { 'Content-Type': 'application/json' };

    if (message.content && typeof message.content === 'object') {
      payload = message.content;
      if (message.headers) {
        headers = { ...headers, ...message.headers };
      }
    }

    try {
      const result = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/messages`, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload)
      });
      return result;
    } catch (error) {
      console.error('Error publishing message:', error);
      throw error;
    }
  },

  async consumeMessages(domainName, queueName, options = {}) {
    const { timeout = 30, max = 10 } = options;
    try {
      const data = await this.fetchJSON(
        `${API_BASE_URL}/domains/${domainName}/queues/${queueName}/messages?timeout=${timeout}&max=${max}`
      );
      return data.messages || [];
    } catch (error) {
      console.error(`Error consuming messages from ${domainName}/${queueName}:`, error);
      return []; // Retourner un tableau vide en cas d'erreur
    }
  },

  async getRoutingRules(domainName) {
    try {
      const data = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/routes`);
      console.log({ data })
      return data.rules || [];
    } catch (error) {
      console.error(`Error fetching routing rules for domain ${domainName}:`, error);
      return []; // Retourner un tableau vide en cas d'erreur
    }
  },

  async addRoutingRule(domainName, rule) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/routes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(rule)
    });
  },

  async deleteRoutingRule(domainName, sourceQueue, destinationQueue) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/routes/${sourceQueue}/${destinationQueue}`, {
      method: 'DELETE'
    });
  },

  // Tester les règles de routage
  async testRouting(domainName, message) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/routes/test`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(message)
    });
  },

  // WebSocket pour le moniteur en temps réel
  createQueueMonitor(domainName, queueName, onMessage, onError) {
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/ws/domains/${domainName}/queues/${queueName}`;

      const socket = new WebSocket(wsUrl);

      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          // Vérifier le format du message et adapter si nécessaire
          if (data.type === 'message' && data.payload) {
            onMessage({
              id: data.id,
              timestamp: data.timestamp,
              headers: data.headers || {},
              content: data.payload,
              ...data.payload // Pour accès direct aux propriétés
            });
          } else {
            onMessage(data);
          }
        } catch (error) {
          console.error('Error parsing WebSocket message:', error);
          if (onError) onError(error);
        }
      };

      socket.onerror = (error) => {
        console.error('WebSocket error:', error);
        if (onError) onError(error);
      };

      return {
        socket,
        close: () => socket.close()
      };
    } catch (error) {
      console.error('Error creating WebSocket:', error);
      if (onError) onError(error);
      // Retourner un objet minimal pour éviter les erreurs
      return {
        socket: null,
        close: () => console.log('No WebSocket connection to close')
      };
    }
  },

  // Statistiques
  async getStats() {
    try {
      return await this.fetchJSON(`${API_BASE_URL}/stats`);
    } catch (error) {
      console.error('Error fetching stats:', error);
      // Retourner des données par défaut en cas d'erreur
      return {
        domains: 0,
        queues: 0,
        messages: 0,
        routes: 0,
        messageRates: []
      };
    }
  },

  // Vérification de santé
  async healthCheck() {
    try {
      return await this.fetchJSON('/health');
    } catch (error) {
      console.error('Health check failed:', error);
      return { status: 'error' };
    }
  }
};

export default api;