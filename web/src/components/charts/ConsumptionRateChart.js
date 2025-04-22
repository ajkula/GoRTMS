import React from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, Area, ComposedChart } from 'recharts';

const ConsumptionRateChart = ({ data }) => {
  // Traiter les données pour le graphique
  const chartData = data.map(item => {
    const date = new Date(item.timestamp * 1000);
    const timeString = `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
    
    return {
      time: timeString,
      timestamp: item.timestamp,
      published: item.published || 0,
      consumed: item.consumed || 0,
      // Calculer le différentiel (positif = plus de messages publiés que consommés)
      differential: (item.published || 0) - (item.consumed || 0)
    };
  });

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Publish vs Consume Rates</h2>
      
      {chartData.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart
              data={chartData}
              margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis 
                dataKey="time" 
                tickFormatter={(time) => time}
              />
              <YAxis />
              <Tooltip 
                labelFormatter={(value) => {
                  const item = chartData.find(d => d.time === value);
                  const date = new Date(item.timestamp * 1000);
                  return `Time: ${date.toLocaleTimeString()}`;
                }}
                formatter={(value, name) => {
                  const label = name === 'published' ? 'Published' : 
                                name === 'consumed' ? 'Consumed' : 
                                'Differential';
                  return [value.toFixed(2), label];
                }}
              />
              <Legend />
              <Area 
                type="monotone" 
                dataKey="differential" 
                fill="#8884d8" 
                stroke="#8884d8"
                fillOpacity={0.3}
                name="Differential"
              />
              <Line 
                type="monotone" 
                dataKey="published" 
                stroke="#ff7300" 
                activeDot={{ r: 8 }}
                strokeWidth={2}
                name="Published"
                dot={false}
              />
              <Line 
                type="monotone" 
                dataKey="consumed" 
                stroke="#4CAF50" 
                activeDot={{ r: 8 }}
                strokeWidth={2}
                name="Consumed"
                dot={false}
              />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="h-64 flex items-center justify-center text-gray-500">
          <p>No consumption rate data available.</p>
        </div>
      )}
    </div>
  );
};

export default ConsumptionRateChart;
