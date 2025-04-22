import React, { useState, useEffect } from 'react';
import { ResponsiveContainer, ComposedChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, Cell } from 'recharts';
import { stringToColor } from '../../utils/utils';
import api from '../../api';

const DomainUsageChart = () => {
  const [domainData, setDomainData] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  
  const loadDomainData = async () => {
    try {
      setLoading(true);
      const currentStats = await api.getCurrentStats();
      
      if (currentStats && currentStats.domainStats) {
        // Transformer les données de domaine pour le graphique
        const chartData = Object.entries(currentStats.domainStats).map(([domainName, stats]) => ({
          name: domainName,
          messageCount: stats.messageCount,
          queueCount: stats.queueCount,
          memoryUsage: Math.round(stats.estimatedMemory / (1024 * 1024)), // Convertir en MB
          color: stringToColor(domainName)
        }));
        
        setDomainData(chartData);
      }
    } catch (err) {
      console.error('Error loading domain usage data:', err);
      setError('Failed to load domain usage data');
    } finally {
      setLoading(false);
    }
  };
  
  useEffect(() => {
    loadDomainData();
    
    const interval = setInterval(loadDomainData, 30000);
    return () => clearInterval(interval);
  }, []);
  
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Domain Resource Usage</h2>
      
      {loading && domainData.length === 0 && (
        <div className="flex justify-center items-center h-64">
          <p className="text-gray-500">Loading domain data...</p>
        </div>
      )}
      
      {error && (
        <div className="bg-red-50 border-l-4 border-red-500 p-3 mb-4">
          <p className="text-red-700 text-sm">{error}</p>
        </div>
      )}
      
      {domainData.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart
              data={domainData}
              margin={{ top: 10, right: 30, left: 10, bottom: 5 }}
              barSize={20}
            >
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis 
                dataKey="name" 
                scale="band" 
                tick={{ fontSize: 12 }}
                tickFormatter={(value) => value.length > 12 ? `${value.substring(0, 10)}...` : value}
              />
              <YAxis 
                yAxisId="left"
                orientation="left"
                label={{ value: 'Count', angle: -90, position: 'insideLeft' }} 
              />
              <YAxis 
                yAxisId="right"
                orientation="right"
                label={{ value: 'Memory (MB)', angle: -90, position: 'insideRight' }} 
              />
              <Tooltip 
                content={({ active, payload, label }) => {
                  if (active && payload && payload.length) {
                    return (
                      <div className="bg-white p-3 border rounded shadow">
                        <p className="font-bold">{label}</p>
                        <p className="text-sm">Messages: <span className="font-medium">{payload[0].value.toLocaleString()}</span></p>
                        <p className="text-sm">Queues: <span className="font-medium">{payload[1].value}</span></p>
                        <p className="text-sm">Memory: <span className="font-medium">{payload[2].value} MB</span></p>
                      </div>
                    );
                  }
                  return null;
                }}
              />
              <Legend />
              <Bar 
                yAxisId="left"
                dataKey="messageCount" 
                name="Messages" 
                fill="#8884d8"
                radius={[4, 4, 0, 0]}
              >
                {domainData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Bar>
              <Bar 
                yAxisId="left"
                dataKey="queueCount" 
                name="Queues" 
                fill="#82ca9d" 
                radius={[4, 4, 0, 0]}
              />
              <Bar 
                yAxisId="right"
                dataKey="memoryUsage" 
                name="Memory (MB)" 
                fill="#ff7300" 
                radius={[4, 4, 0, 0]}
              />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
      ) : !loading && (
        <div className="h-64 flex items-center justify-center text-gray-500">
          <p>No domain data available.</p>
        </div>
      )}
    </div>
  );
};

export default DomainUsageChart;
