import React, { useEffect, useState, useRef } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, ReferenceLine } from 'recharts';

const QueueFillRatesChart = ({ data }) => {
  const [margin, setMargin] = useState({ top: 5, right: 30, left: 30, bottom: 5 });
  const [displayData, setDisplayData] = useState([]);
  const chartRef = useRef(null);

  useEffect(() => {
    // Déterminer le nombre maximum d'éléments à afficher en fonction de la hauteur
    const MAX_ITEMS = 10;
    const MIN_MARGIN_LEFT = 30;
    const MAX_NAME_LENGTH = 85;
    
    // Trier et limiter les données
    const sortedData = [...data]
      .sort((a, b) => b.usage - a.usage)
      .map(queue => ({
        name: queue.name,
        domain: queue.domain,
        usage: queue.usage || 0,
        messageCount: queue.messageCount || 0,
        maxSize: queue.maxSize || 0,
        // Calculer la longueur "affichable" du nom
        displayName: queue.name.length > MAX_NAME_LENGTH ? 
          `${queue.name.substring(0, MAX_NAME_LENGTH - 3)}...` : 
          queue.name
      }));

    // Déterminer combien d'éléments afficher (basé sur le remplissage)
    const chartData = sortedData.slice(0, MAX_ITEMS);
    
    // Calculer la marge gauche en fonction du nom le plus long
    const longestNameLength = Math.max(...chartData.map(item => item.displayName.length));
    // Estimation: ~6px par caractère (peut être ajusté selon la police)
    const calculatedMargin = Math.max(MIN_MARGIN_LEFT, Math.min(200, longestNameLength * 6));
    
    setMargin({ ...margin, left: calculatedMargin });
    setDisplayData(chartData);
  }, [data]);

  const getBarFill = (percent) => {
    if (percent >= 90) return '#ef4444'; // Red
    if (percent >= 75) return '#f97316'; // Orange
    if (percent >= 50) return '#eab308'; // Yellow
    return '#22c55e';                    // Green
  };

  const CustomizedLegend = (props) => {
    const { payload } = props;
    
    return (
      <div className="flex items-center justify-center mt-2">
        {payload.map((entry, index) => (
          <div key={`item-${index}`} className="flex items-center mx-2">
            <div 
              className="w-3 h-3 mr-1" 
              style={{ backgroundColor: entry.color }}
            />
            <span className="text-sm">Utilization %</span>
          </div>
        ))}
      </div>
    );
  };

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Queue Fill Rates</h2>
      
      {displayData.length > 0 ? (
        <div className="h-64" ref={chartRef}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={displayData}
              layout="vertical"
              margin={margin}
            >
              <CartesianGrid strokeDasharray="3 3" horizontal={true} vertical={false} />
              <XAxis 
                type="number" 
                domain={[0, 100]} 
                tickFormatter={(value) => `${value}%`}
              />
              <YAxis 
                dataKey="displayName" 
                type="category" 
                tickLine={false}
                width={margin.left - 10}
                tick={{ 
                  textAnchor: 'end',
                  fontSize: 12,
                  fill: '#666',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis'
                }}
              />
              <Tooltip 
                formatter={(value) => [`${value.toFixed(1)}%`, 'Utilization']}
                labelFormatter={(value) => {
                  const item = displayData.find(d => d.displayName === value);
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
              <Legend content={<CustomizedLegend />} />
              <ReferenceLine x={75} stroke="#f97316" strokeDasharray="3 3" label={{ value: 'Warning', position: 'insideBottomRight', fill: '#f97316' }} />
              <ReferenceLine x={90} stroke="#ef4444" strokeDasharray="3 3" label={{ value: 'Critical', position: 'insideBottom', fill: '#ef4444' }} />
              <Bar 
                dataKey="usage" 
                name="Utilization %" 
                fill="#8884d8"
                animationDuration={500}
                isAnimationActive={true}
                animationEasing="ease-in-out"
                barSize={20}
                radius={[0, 4, 4, 0]}
                cell={({ payload }) => (
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
