import React, { useMemo } from 'react';
import { ResponsiveContainer, Treemap, Tooltip } from 'recharts';

const DomainQueueTreeMap = ({ data }) => {
  // Transformer les données pour le treemap
  // Grouper les queues par domaine
  const treemapData = useMemo(() => {
    const domains = {};
    
    // Grouper les queues par domaine
    data.forEach(queue => {
      if (!domains[queue.domain]) {
        domains[queue.domain] = {
          name: queue.domain,
          children: []
        };
      }
      
      domains[queue.domain].children.push({
        name: queue.name,
        size: queue.messageCount || 1, // Utiliser au moins 1 pour éviter les zones vides
        usage: queue.usage
      });
    });
    
    // Convertir en tableau pour recharts
    return Object.values(domains);
  }, [data]);

  // Couleurs basées sur l'utilisation
  const getColor = (usage) => {
    if (usage >= 90) return '#ef4444'; // Rouge pour utilisation critique
    if (usage >= 75) return '#f97316'; // Orange pour utilisation élevée
    if (usage >= 50) return '#eab308'; // Jaune pour utilisation moyenne
    return '#22c55e';                  // Vert pour utilisation faible
  };

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Queues by Domain</h2>
      
      {treemapData.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <Treemap
              data={treemapData}
              dataKey="size"
              aspectRatio={4/3}
              stroke="#fff"
              fill="#8884d8"
              animationDuration={500}
              content={({ root, depth, x, y, width, height, index, payload, colors, rank, name }) => {
                // Ne rendre que les enfants, pas les parents
                if (depth === 1) {
                  const queue = root.children[index];
                  return (
                    <g>
                      <rect
                        x={x}
                        y={y}
                        width={width}
                        height={height}
                        style={{
                          fill: getColor(queue.usage),
                          stroke: '#fff',
                          strokeWidth: 2 / (depth + 1e-10),
                          strokeOpacity: 1 / (depth + 1e-10),
                        }}
                      />
                      {width > 30 && height > 20 && (
                        <text
                          x={x + width / 2}
                          y={y + height / 2 + 7}
                          textAnchor="middle"
                          fill="#fff"
                          fontSize={12}
                          fontWeight="300"
                        >
                          {name}
                        </text>
                      )}
                    </g>
                  );
                }
                return null;
              }}
            >
              <Tooltip 
                content={({ payload }) => {
                  if (payload && payload.length > 0) {
                    const data = payload[0].payload;
                    return (
                      <div className="bg-white p-2 border rounded shadow">
                        <p className="font-bold">{data.name}</p>
                        <p>Domain: {data.root?.name || 'Unknown'}</p>
                        <p>Messages: {data.size.toLocaleString()}</p>
                        {data.usage && <p>Usage: {data.usage.toFixed(1)}%</p>}
                      </div>
                    );
                  }
                  return null;
                }}
              />
            </Treemap>
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

export default DomainQueueTreeMap;
