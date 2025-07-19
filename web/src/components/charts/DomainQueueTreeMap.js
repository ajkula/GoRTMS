import React, { useMemo } from 'react';
import { ResponsiveContainer, Treemap, Tooltip } from 'recharts';

// Function to get the color based on usage
const getUsageColor = (usage) => {
  if (usage >= 90) return '#ef4444'; // Red for critical usage
  if (usage >= 75) return '#f97316'; // Orange for high usage
  if (usage >= 50) return '#eab308'; // Yellow for medium usage
  return '#22c55e';                  // Green for low usage
};

const DomainQueueTreeMap = ({ data }) => {
  // Transform data for the treemap
  const treemapData = useMemo(() => {
    // using a flatter format with computed colors
    return data.map(queue => ({
      name: queue.name,
      size: queue.messageCount || 1,
      domain: queue.domain,
      usage: queue.usage || 0,
      fill: getUsageColor(queue.usage || 0)
    }));
  }, [data]);

  // Get unique domains for the legends
  const domains = useMemo(() => {
    const uniqueDomains = [...new Set(data.map(queue => queue.domain))];
    return uniqueDomains;
  }, [data]);

  // Generate the usage legend
  const usageLegend = [
    { label: "Critical (>90%)", color: getUsageColor(95) },
    { label: "High (>75%)", color: getUsageColor(80) },
    { label: "Medium (>50%)", color: getUsageColor(60) },
    { label: "Low (<50%)", color: getUsageColor(40) }
  ];

  return (
    <div>
      <h2 className="text-lg font-medium text-gray-900 mb-4">Queues by Domain</h2>

      {treemapData.length > 0 ? (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <Treemap
              data={treemapData}
              dataKey="size"
              nameKey="name"
              stroke="#fff"
              // Use the style property to specify the colors
              content={({
                x, y, width, height, name, fill
              }) => (
                <g>
                  <rect
                    x={x}
                    y={y}
                    width={width}
                    height={height}
                    style={{
                      fill,
                      stroke: '#fff',
                      strokeWidth: 1,
                      strokeOpacity: 0.8
                    }}
                  />
                  {width > 35 && height > 20 && (
                    <text
                      x={x + width / 2}
                      y={y + height / 2}
                      textAnchor="middle"
                      dominantBaseline="middle"
                      fill="#fff"
                      fontSize={10}
                      fontWeight="bold"
                    >
                      {name}
                    </text>
                  )}
                </g>
              )}
            >
              <Tooltip
                formatter={(value) => value ? [`${value} messages`, 'Messages'] : [0, 'Messages']}
                labelFormatter={(label) => label ? `Queue: ${label}` : 'Unknown'}
                content={({ payload }) => {
                  if (payload && payload.length > 0) {
                    const data = payload[0].payload;
                    return (
                      <div className="bg-white p-2 border rounded shadow">
                        <p className="font-bold">{data.name}</p>
                        <p>Domain: {data.domain || 'Unknown'}</p>
                        <p>Messages: {(data.size || 0).toLocaleString()}</p>
                        <p>Usage: {(data.usage || 0).toFixed(1)}%</p>
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

      {/* Inline usage legend */}
      <div className="mt-4 text-center">
        <div className="inline-block text-center">
          <span className="text-sm font-medium text-gray-700">Relative usage:</span>&nbsp;&nbsp;
          {usageLegend.map((item, index) => (
            <span key={item.label} className="inline-flex items-center mx-2">
              <div
                className="w-4 h-4 inline-block mr-1"
                style={{ backgroundColor: item.color }}
              ></div>
              <span className="text-sm text-gray-700">{item.label}</span>
            </span>
          ))}
        </div>
      </div>

      {/* Inline domain legend */}
      {domains.length > 0 && (
        <div className="mt-3 text-center">
          <div className="inline-block text-center">
            <span className="text-sm font-medium text-gray-700">Domains:</span>&nbsp;&nbsp;
            {domains.map((domain, index) => (
              <span key={index} className="inline-flex items-center mx-2">
                <span className="text-sm text-gray-700">{domain}</span>
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default DomainQueueTreeMap;
