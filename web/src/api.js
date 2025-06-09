const API_BASE_URL = '/api';

const api = {
  // Utility function to handle errors and parse JSON responses
  async fetchJSON(url, options = {}) {
    if (!url) {
      console.error('API fetch error: URL is undefined');
      throw new Error('API fetch error: URL is undefined');
    }

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

  async getDomains() {
    try {
      const data = await this.fetchJSON(`${API_BASE_URL}/domains`);
      return data.domains || [];
    } catch (error) {
      console.error('Error fetching domains:', error);
      return [];
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

  async getQueues(domainName) {
    try {
      const data = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues`);
      return data.queues || [];
    } catch (error) {
      console.error(`Error fetching queues for domain ${domainName}:`, error);
      return [];
    }
  },

  async getQueue(domainName, queueName) {
    const data = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}`);
    return {
      name: data.name,
      messageCount: data.messageCount,
      config: data.config || {}
    };
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
      // This API is only for initial registration
      const response = await fetch(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/subscribe`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({}),
      });

      if (!response.ok) {
        throw new Error(`Error subscribing to queue: ${response.statusText}`);
      }
      const data = await response.json();
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
        body: JSON.stringify({ subscriptionId }),
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
      return [];
    }
  },

  async getRoutingRules(domainName) {
    try {
      const data = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/routes`);
      console.log({ data })
      return data.rules || [];
    } catch (error) {
      console.error(`Error fetching routing rules for domain ${domainName}:`, error);
      return [];
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

  async testRouting(domainName, message) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/routes/test`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(message)
    });
  },

  // real-time monitoring webSocket
  createQueueMonitor(domainName, queueName, onMessage, onError) {
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/ws/domains/${domainName}/queues/${queueName}`;

      const socket = new WebSocket(wsUrl);

      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === 'message' && data.payload) {
            onMessage({
              id: data.id,
              timestamp: data.timestamp,
              headers: data.headers || {},
              content: data.payload,
              ...data.payload
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
      return {
        socket: null,
        close: () => console.log('No WebSocket connection to close')
      };
    }
  },

  async getStats(params = {}) {
    try {
    const queryString = new URLSearchParams(params).toString();
    const url = `${API_BASE_URL}/stats${queryString ? `?${queryString}` : ''}`;
      return await this.fetchJSON(url);
    } catch (error) {
      console.error('Error fetching stats:', error);
      return {
        domains: 0,
        queues: 0,
        messages: 0,
        routes: 0,
        messageRates: []
      };
    }
  },

  async getCurrentStats() {
    try {
      return await this.fetchJSON(`${API_BASE_URL}/resources/current`);
    } catch (error) {
      console.error('Error fetching current resource stats:', error);
      return {
        timestamp: Date.now() / 1000,
        memoryUsage: 0,
        goroutines: 0,
        gcCycles: 0,
        gcPauseNs: 0,
        heapObjects: 0,
        domainStats: {}
      };
    }
  },

  async getStatsHistory(limit = 60) {
    try {
      return await this.fetchJSON(`/api/resources/history?limit=${limit}`);
    } catch (error) {
      console.error('Error fetching resource stats history:', error);
      return [];
    }
  },

  async getDomainStats(domainName) {
    try {
      return await this.fetchJSON(`/api/resources/domains/${domainName}`);
    } catch (error) {
      console.error(`Error fetching domain resource stats for ${domainName}:`, error);
      return {
        queueCount: 0,
        messageCount: 0,
        queueStats: {},
        estimatedMemory: 0
      };
    }
  },

  // Consumer Groups
  async getAllConsumerGroups() {
    return this.fetchJSON(`${API_BASE_URL}/consumer-groups`);
  },

  async getConsumerGroups(domainName, queueName) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups`);
  },

  async getConsumerGroup(domainName, queueName, groupID) {
    try {
      console.log(`Fetching consumer group: ${domainName}/${queueName}/${groupID}`);
      const data = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups/${groupID}`);
      console.log("Consumer group data received:", data);
      return data;
    } catch (error) {
      console.error(`Error fetching consumer group ${groupID}:`, error);
      throw error;
    }
  },

  async createConsumerGroup(domainName, queueName, groupConfig) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(groupConfig)
    });
  },

  async deleteConsumerGroup(domainName, queueName, groupID) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups/${groupID}`, {
      method: 'DELETE'
    });
  },

  async updateConsumerGroupTTL(domainName, queueName, groupID, ttl) {
    try {
      console.log(`Updating TTL for ${domainName}/${queueName}/${groupID} to ${ttl}`);
      const result = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups/${groupID}/ttl`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ttl })
      });
      console.log("TTL update result:", result);
      return result;
    } catch (error) {
      console.error(`Error updating TTL for group ${groupID}:`, error);
      throw error;
    }
  },

  async getPendingMessages(domainName, queueName, groupID) {
    try {
      console.log(`Fetching pending messages: ${domainName}/${queueName}/${groupID}`);
      const data = await this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups/${groupID}/messages`);
      console.log("Pending messages received:", data);
      return Array.isArray(data) ? data : (data.messages || []);
    } catch (error) {
      console.error(`Error fetching pending messages for group ${groupID}:`, error);
      return [];
    }
  },

  // TODO
  // async acknowledgeMessage(domainName, queueName, groupID, messageID) {
  //   return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups/${groupID}/messages/${messageID}/ack`, {
  //     method: 'POST'
  //   });
  // },

  async addConsumerToGroup(domainName, queueName, groupID, consumerID) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups/${groupID}/consumers`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ consumerID })
    });
  },

  async removeConsumerFromGroup(domainName, queueName, groupID, consumerID) {
    return this.fetchJSON(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/consumer-groups/${groupID}/consumers/${consumerID}`, {
      method: 'DELETE'
    });
  },

  // Format data for charts
  formatHistoryForCharts(historyData) {
    return historyData.map(stats => ({
      time: new Date(stats.timestamp * 1000).toLocaleTimeString(),
      timestamp: stats.timestamp,
      memoryUsageMB: Math.round(stats.memoryUsage / (1024 * 1024)), // convert to MB
      goroutines: stats.goroutines,
      gcPauseMs: stats.gcPauseNs / 1000000, // convert to ms
      heapObjects: stats.heapObjects
    }));
  },

  // Format memory size for display
  formatMemorySize(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  },

  async healthCheck() {
    try {
      return await this.fetchJSON('/health');
    } catch (error) {
      console.error('Health check failed:', error);
      return { status: 'error' };
    }
  },


  // Mocks TODO implem
  getNotifications: async () => {
    // Simulate network delay
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // Mock data - TODO: Replace with actual API call
    return [
      {
        id: '1',
        type: 'warning',
        message: 'Queue orders.processing is approaching capacity',
        timestamp: Date.now() - 5 * 60 * 1000,
        read: false
      },
      {
        id: '2',
        type: 'info',
        message: 'New domain analytics created',
        timestamp: Date.now() - 60 * 60 * 1000,
        read: false
      }
    ];
  },

  markNotificationAsRead: async (notificationId) => {
    // Simulate network delay
    await new Promise(resolve => setTimeout(resolve, 50));
    
    // TODO: Replace with actual API call
    // For now, just return success
    return { success: true };
  },

  // Settings
  async getSettings() {
    try {
      const response = await fetch('/api/settings', {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || `HTTP ${response.status}: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to get settings:', error);
      throw error;
    }
  },

  async updateSettings(config) {
    try {
      const response = await fetch('/api/settings', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ config }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || `HTTP ${response.status}: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to update settings:', error);
      throw error;
    }
  },

  async resetSettings() {
    try {
      const response = await fetch('/api/settings/reset', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || `HTTP ${response.status}: ${response.statusText}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Failed to reset settings:', error);
      throw error;
    }
  },
};

export default api;