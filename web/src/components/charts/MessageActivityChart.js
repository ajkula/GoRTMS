import React from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';

const MessageActivityChart = ({ data }) => {
  // Prétraiter les données pour éviter les erreurs
  const processedData = data.map(item => {
    // S'assurer que chaque élément a un timestamp valide
    return {
      ...item,
      // Conserver le timestamp brut pour l'axe X
      timestamp: item.timestamp || Date.now() / 1000, // Utiliser l'heure actuelle si non défini
      published: item.published || 0,
      consumed: item.consumed || 0
    };
  });

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Message Activity</h2>
      
      {processedData.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={processedData}
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
                  if (value === undefined || value === null) {
                    return 'Unknown time';
                  }
                  const date = new Date(value * 1000);
                  return `Time: ${date.toLocaleTimeString()}`;
                }}
                formatter={(value, name) => {
                  const val = value || 0;
                  return [val.toFixed(2), name];
                }}
              />
              <Legend />
              <Bar dataKey="published" fill="#8884d8" name="Published" />
              <Bar dataKey="consumed" fill="#82ca9d" name="Consumed" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="h-64 flex items-center justify-center text-gray-500">
          <p>No message activity data available.</p>
        </div>
      )}
    </div>
  );
};

export default MessageActivityChart;
