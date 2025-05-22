import React from 'react';
import { ResponsiveContainer, CartesianGrid, Tooltip, Legend, Area, AreaChart, Line, YAxis, XAxis } from 'recharts';
import { RefreshCw, Database, Server, AlertTriangle, Loader } from 'lucide-react';
import { useResourceStats } from '../../hooks/useResourceStats';
import api from '../../api';

const ResourceMonitor = () => {
  // Use the custom hook to manage resource data
  const { 
    chartData, 
    currentStats, 
    loading, 
    error, 
    refresh 
  } = useResourceStats(30, 30000);

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-lg font-medium text-gray-900">System Resources</h2>
        <div className="flex space-x-2">
          <button
            onClick={refresh}
            className="inline-flex items-center px-3 py-1 border border-gray-300 shadow-sm text-sm rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            disabled={loading}
          >
            {loading ? (
              <Loader className="h-3 w-3 animate-spin mr-1" />
            ) : (
              <RefreshCw className="h-3 w-3 mr-1" />
            )}
            Refresh
          </button>
          <div className="inline-flex items-center px-2 py-1 bg-blue-100 text-blue-800 text-xs rounded-full">
            <Database className="h-3 w-3 mr-1" />
            <span>Memory</span>
          </div>
          <div className="inline-flex items-center px-2 py-1 bg-green-100 text-green-800 text-xs rounded-full">
            <Server className="h-3 w-3 mr-1" />
            <span>Goroutines</span>
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-4 bg-red-50 border-l-4 border-red-500 p-3 rounded">
          <div className="flex">
            <AlertTriangle className="h-5 w-5 text-red-500 mr-2" />
            <div>
              <p className="text-sm text-red-700">{error}</p>
            </div>
          </div>
        </div>
      )}

      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart
            data={chartData}
            margin={{ top: 10, right: 30, left: 0, bottom: 0 }}
          >
            <defs>
              <linearGradient id="memColor" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#3b82f6" stopOpacity={0.1} />
              </linearGradient>
              <linearGradient id="routinesColor" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#22c55e" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#22c55e" stopOpacity={0.1} />
              </linearGradient>
              <linearGradient id="gcColor" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#ef4444" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#ef4444" stopOpacity={0.1} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis 
              dataKey="timestamp" 
              tickFormatter={(unixTime) => {
                if (!unixTime) return '';
                const date = new Date(unixTime * 1000);
                return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
              }}
              scale="time"
              type="number"
              domain={['dataMin', 'dataMax']}
            />
            <YAxis yAxisId="left" orientation="left" stroke="#3b82f6" />
            <YAxis yAxisId="right" orientation="right" stroke="#22c55e" />
            <Tooltip 
              formatter={(value, name) => {
                if (value === undefined || value === null) return [0, name];
                
                if (name === "Memory (MB)") return [value.toFixed(2), "Memory (MB)"];
                if (name === "Goroutines") return [value, "Goroutines"];
                if (name === "GC Pause") return [value.toFixed(2), "GC Pause (ms)"];
                return [value, name];
              }}
              labelFormatter={(time) => {
                if (!time) return "Unknown time";
                return `Time: ${time}`;
              }}
            />
            <Legend />
            <Area 
              yAxisId="left"
              type="monotone" 
              dataKey="memoryUsageMB" 
              name="Memory (MB)" 
              stroke="#3b82f6" 
              fillOpacity={1} 
              fill="url(#memColor)" 
            />
            <Area 
              yAxisId="right"
              type="monotone" 
              dataKey="goroutines" 
              name="Goroutines" 
              stroke="#22c55e" 
              fillOpacity={1} 
              fill="url(#routinesColor)" 
            />
            <Line 
              yAxisId="left"
              type="monotone" 
              dataKey="gcPauseMs" 
              name="GC Pause" 
              stroke="#ef4444" 
              strokeWidth={2}
              dot={false}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      <div className="mt-4 grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="border rounded-lg p-3">
          <p className="text-sm text-gray-500">Current Memory</p>
          <p className="text-lg font-bold">
            {currentStats ? 
              api.formatMemorySize(currentStats.memoryUsage) : 
              chartData[chartData.length - 1].memoryUsageMB + ' MB'}
          </p>
        </div>
        <div className="border rounded-lg p-3">
          <p className="text-sm text-gray-500">Goroutines</p>
          <p className="text-lg font-bold">
            {currentStats ? 
              currentStats.goroutines : 
              chartData[chartData.length - 1].goroutines}
          </p>
        </div>
        <div className="border rounded-lg p-3">
          <p className="text-sm text-gray-500">GC Pause</p>
          <p className="text-lg font-bold">
            {currentStats ? 
              (currentStats.gcPauseNs / 1000000).toFixed(2) + ' ms' : 
              chartData[chartData.length - 1].gcPauseMs.toFixed(2) + ' ms'}
          </p>
        </div>
        <div className="border rounded-lg p-3">
          <p className="text-sm text-gray-500">Heap Objects</p>
          <p className="text-lg font-bold">
            {currentStats ? 
              currentStats.heapObjects.toLocaleString() : 
              '0'}
          </p>
        </div>
      </div>
      
      {!currentStats && (
        <div className="mt-4 text-sm text-gray-500">
          <p>Note: These are simulated values. Make sure to implement the resource monitoring service on the server side.</p>
        </div>
      )}
    </div>
  );
};

export default ResourceMonitor;
