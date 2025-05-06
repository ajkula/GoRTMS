import React from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';

const MessageActivityChart = ({ data }) => {
  // Convertir les données pour utiliser des chaînes de temps formatées comme clé
  const processedData = data.map(item => {
    const date = new Date(item.timestamp * 1000);
    const timeString = `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
    
    return {
      time: timeString, // Utiliser une chaîne de temps formatée au lieu du timestamp brut
      published: item.publishedTotal || 0,
      consumed: item.consumedTotal || 0,
      // Conserver le timestamp original pour le tooltip
      timestamp: item.timestamp
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
              <XAxis dataKey="time" /> {/* Utiliser la chaîne de temps formatée */}
              <YAxis />
              <Tooltip 
                labelFormatter={(value) => {
                  const item = processedData.find(d => d.time === value);
                  if (item && item.timestamp) {
                    const date = new Date(item.timestamp * 1000);
                    return `Time: ${date.toLocaleTimeString()}`;
                  }
                  return `Time: ${value}`;
                }}
                formatter={(value, name) => {
                  const val = value || 0;
                  return [val.toFixed(0), name];
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