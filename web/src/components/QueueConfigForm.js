import React from 'react';

const QueueConfigForm = ({ queueConfig, setQueueConfig }) => {
  return (
    <div className="mt-4 bg-gray-50 p-4 rounded-md">
      <h3 className="text-sm font-medium text-gray-700 mb-3">Queue Configuration</h3>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Persistence
          </label>
          <select
            value={queueConfig.isPersistent ? "true" : "false"}
            onChange={(e) => setQueueConfig({ ...queueConfig, isPersistent: e.target.value === "true" })}
            className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          >
            <option value="true">Persistent</option>
            <option value="false">Temporary</option>
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Delivery Mode
          </label>
          <select
            value={queueConfig.deliveryMode}
            onChange={(e) => setQueueConfig({ ...queueConfig, deliveryMode: e.target.value })}
            className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          >
            <option value="broadcast">Broadcast</option>
            <option value="roundRobin">Round Robin</option>
            <option value="singleConsumer">Single Consumer</option>
          </select>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Max Size
          </label>
          <input
            type="number"
            value={queueConfig.maxSize}
            onChange={(e) => setQueueConfig({ ...queueConfig, maxSize: parseInt(e.target.value) })}
            className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            TTL (Time To Live)
          </label>
          <input
            type="text"
            value={queueConfig.ttl}
            onChange={(e) => setQueueConfig({ ...queueConfig, ttl: e.target.value })}
            placeholder="e.g., 86400s, 24h"
            className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>
      </div>

      {/* Retry Configuration */}
      <div className="mb-4">
        <div className="flex items-center mb-2">
          <input
            type="checkbox"
            id="retryEnabled"
            checked={queueConfig.retryEnabled}
            onChange={(e) => setQueueConfig({ ...queueConfig, retryEnabled: e.target.checked })}
            className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
          />
          <label htmlFor="retryEnabled" className="ml-2 block text-sm font-medium text-gray-700">
            Enable Message Retry
          </label>
        </div>

        {queueConfig.retryEnabled && (
          <div className="ml-6 mt-2 grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Max Retries
              </label>
              <input
                type="number"
                value={queueConfig.retryConfig.maxRetries}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  retryConfig: {
                    ...queueConfig.retryConfig,
                    maxRetries: parseInt(e.target.value)
                  }
                })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Initial Delay
              </label>
              <input
                type="text"
                value={queueConfig.retryConfig.initialDelay}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  retryConfig: {
                    ...queueConfig.retryConfig,
                    initialDelay: e.target.value
                  }
                })}
                placeholder="e.g., 1s, 500ms"
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Max Delay
              </label>
              <input
                type="text"
                value={queueConfig.retryConfig.maxDelay}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  retryConfig: {
                    ...queueConfig.retryConfig,
                    maxDelay: e.target.value
                  }
                })}
                placeholder="e.g., 30s, 5m"
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Backoff Factor
              </label>
              <input
                type="number"
                step="0.1"
                value={queueConfig.retryConfig.factor}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  retryConfig: {
                    ...queueConfig.retryConfig,
                    factor: parseFloat(e.target.value)
                  }
                })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>
          </div>
        )}
      </div>

      {/* Circuit Breaker Configuration */}
      <div>
        <div className="flex items-center mb-2">
          <input
            type="checkbox"
            id="circuitBreakerEnabled"
            checked={queueConfig.circuitBreakerEnabled}
            onChange={(e) => setQueueConfig({ ...queueConfig, circuitBreakerEnabled: e.target.checked })}
            className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
          />
          <label htmlFor="circuitBreakerEnabled" className="ml-2 block text-sm font-medium text-gray-700">
            Enable Circuit Breaker
          </label>
        </div>

        {queueConfig.circuitBreakerEnabled && (
          <div className="ml-6 mt-2 grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Error Threshold
              </label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="1"
                value={queueConfig.circuitBreakerConfig.errorThreshold}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  circuitBreakerConfig: {
                    ...queueConfig.circuitBreakerConfig,
                    errorThreshold: parseFloat(e.target.value)
                  }
                })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Minimum Requests
              </label>
              <input
                type="number"
                value={queueConfig.circuitBreakerConfig.minimumRequests}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  circuitBreakerConfig: {
                    ...queueConfig.circuitBreakerConfig,
                    minimumRequests: parseInt(e.target.value)
                  }
                })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Open Timeout
              </label>
              <input
                type="text"
                value={queueConfig.circuitBreakerConfig.openTimeout}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  circuitBreakerConfig: {
                    ...queueConfig.circuitBreakerConfig,
                    openTimeout: e.target.value
                  }
                })}
                placeholder="e.g., 30s, 1m"
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">
                Success Threshold
              </label>
              <input
                type="number"
                value={queueConfig.circuitBreakerConfig.successThreshold}
                onChange={(e) => setQueueConfig({
                  ...queueConfig,
                  circuitBreakerConfig: {
                    ...queueConfig.circuitBreakerConfig,
                    successThreshold: parseInt(e.target.value)
                  }
                })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export const defaultQueueConfig = {
  isPersistent: true,
  maxSize: 1000,
  ttl: "86400s",
  deliveryMode: "broadcast",
  retryEnabled: false,
  retryConfig: {
    maxRetries: 3,
    initialDelay: "1s",
    maxDelay: "30s",
    factor: 2.0
  },
  circuitBreakerEnabled: false,
  circuitBreakerConfig: {
    errorThreshold: 0.5,
    minimumRequests: 10,
    openTimeout: "30s",
    successThreshold: 5
  },
  workerCount: 2
};

export default QueueConfigForm;