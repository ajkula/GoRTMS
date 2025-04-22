import React from 'react';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts';

const DomainPieChart = ({ data }) => {
  const colors = ['#8884d8', '#82ca9d', '#ffc658', '#ff8042', '#0088fe', '#00C49F', '#FFBB28', '#FF8042'];
  
  // Transformer les données pour le pie chart
  const chartData = data.map((domain, index) => ({
    name: domain.name,
    value: domain.messageCount || 0,
    color: colors[index % colors.length]
  }));

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Messages by Domain</h2>

      {chartData.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={chartData}
                cx="50%"
                cy="50%"
                labelLine={false}
                outerRadius={80}
                fill="#8884d8"
                dataKey="value"
                label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
              >
                {chartData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip 
                formatter={(value) => [value.toLocaleString(), 'Messages']}
              />
            </PieChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="h-64 flex items-center justify-center text-gray-500">
          <p>No domain data available.</p>
        </div>
      )}
    </div>
  );
};

export default DomainPieChart;
