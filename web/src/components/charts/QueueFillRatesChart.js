import React from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, ReferenceLine } from 'recharts';

const QueueFillRatesChart = ({ data }) => {
  // Limiter aux 10 premières queues pour la lisibilité
  const chartData = data
    .slice(0, 10)
    .map(queue => ({
      name: queue.name,
      domain: queue.domain,
      usage: queue.usage || 0,
      messageCount: queue.messageCount || 0,
      maxSize: queue.maxSize || 0
    }))
    .sort((a, b) => b.usage - a.usage); // Trier par usage décroissant

  // Fonction pour déterminer la couleur en fonction du taux de remplissage
  const getBarFill = (percent) => {
    if (percent >= 90) return '#ef4444'; // Rouge
    if (percent >= 75) return '#f97316'; // Orange
    if (percent >= 50) return '#eab308'; // Jaune
    return '#22c55e';                    // Vert
  };

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Queue Fill Rates</h2>
      
      {chartData.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={chartData}
              layout="vertical"
              margin={{ top: 5, right: 30, left: 85, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray="3 3" horizontal={true} vertical={false} />
              <XAxis 
                type="number" 
                domain={[0, 100]} 
                tickFormatter={(value) => `${value}%`}
              />
              <YAxis 
                dataKey="name" 
                type="category" 
                tickLine={false}
                tickFormatter={(value) => value.length > 15 ? `${value.substring(0, 13)}...` : value}
              />
              <Tooltip 
                formatter={(value, name) => [`${value.toFixed(1)}%`, 'Utilization']}
                labelFormatter={(value) => {
                  const item = chartData.find(d => d.name === value);
                  return `${item.name} (${item.domain})`;
                }}
                content={({ active, payload }) => {
                  if (active && payload && payload.length) {
                    const data = payload[0].payload;
                    return (
                      <div className="bg-white p-2 border rounded shadow">
                        <p className="font-bold">{data.name}</p>
                        <p>Domain: {data.domain}</p>
                        <p>Usage: {data.usage.toFixed(1)}%</p>
                        <p>Messages: {data.messageCount.toLocaleString()} / {data.maxSize.toLocaleString()}</p>
                      </div>
                    );
                  }
                  return null;
                }}
              />
              <Legend />
              <ReferenceLine x={75} stroke="#f97316" strokeDasharray="3 3" label={{ value: 'Warning', position: 'insideBottomRight', fill: '#f97316' }} />
              <ReferenceLine x={90} stroke="#ef4444" strokeDasharray="3 3" label={{ value: 'Critical', position: 'insideBottom', fill: '#ef4444' }} />
              <Bar 
                dataKey="usage" 
                name="Utilization %" 
                fill="#8884d8"
                animationDuration={500}
                // Colorer chaque bar selon son niveau d'utilisation
                isAnimationActive={true}
                animationEasing="ease-in-out"
                barSize={20}
                radius={[0, 4, 4, 0]}
                // cell pour personnaliser chaque barre
                cell={({ payload, index }) => (
                  <rect 
                    fill={getBarFill(payload.usage)} 
                    x={0} y={0} width="100%" height="100%" 
                  />
                )}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="h-64 flex items-center justify-center text-gray-500">
          <p>No queue data available.</p>
        </div>
      )}
    </div>
  );
};

export default QueueFillRatesChart;
