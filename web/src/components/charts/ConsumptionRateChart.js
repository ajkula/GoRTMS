import React, { useMemo } from 'react';
import { XAxis, YAxis, CartesianGrid, Tooltip, Legend, Area, ComposedChart, Line, ResponsiveContainer } from 'recharts';

const ConsumptionRateChart = ({ data }) => {
  // Prétraiter les données pour éviter les erreurs
  const chartData = useMemo(() => {
    return data.map(item => {
      // S'assurer que chaque élément a un timestamp valide
      return {
        ...item,
        timestamp: item.timestamp || Date.now() / 1000, // Utiliser l'heure actuelle si non défini
        published: item.published || 0,
        consumed: item.consumed || 0,
        // Calculer le différentiel (positif = plus de messages publiés que consommés)
        differential: (item.published || 0) - (item.consumed || 0)
      };
    });
  }, [data]);

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
                dataKey="timestamp" 
                tickFormatter={(unixTime) => {
                  const date = new Date(unixTime * 1000);
                  return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
                }}
                scale="time"
                type="number"
                domain={['dataMin', 'dataMax']}
              />
              <YAxis />
              <Tooltip 
                labelFormatter={(value) => {
                  // Vérifier si value est valide
                  if (value === undefined || value === null) return 'Unknown time';
                  const date = new Date(value * 1000);
                  return `Time: ${date.toLocaleTimeString()}`;
                }}
                formatter={(value, name) => {
                  // Valeur par défaut en cas de undefined/null
                  const val = value || 0;
                  return [val.toFixed(2), name];
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
                strokeWidth={2}
                name="Published"
                dot={false}
              />
              <Line 
                type="monotone" 
                dataKey="consumed" 
                stroke="#4CAF50" 
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
