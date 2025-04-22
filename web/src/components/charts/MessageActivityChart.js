import React from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';

const MessageActivityChart = ({ data }) => {
  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Message Activity</h2>
      
      {data.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={data}
              margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis 
                dataKey="time" 
                tickFormatter={(unixTime) => {
                  const date = new Date(unixTime * 1000);
                  return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
                }}
              />
              <YAxis />
              <Tooltip 
                labelFormatter={(value) => {
                  const date = new Date(value * 1000);
                  return `Time: ${date.toLocaleTimeString()}`;
                }}
                formatter={(value, name) => {
                  return [value.toFixed(2), name === 'published' ? 'Published' : 'Consumed'];
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
