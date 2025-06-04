import React from 'react';
import { RotateCcw, Clock, Activity } from 'lucide-react';

const MessageRateControls = ({ 
  period, 
  granularity, 
  isExploring,
  periods,
  availableGranularities,
  onPeriodChange,
  onGranularityChange,
  onReset
}) => {
  return (
    <div className="flex flex-wrap items-center gap-4 mb-4">
      {/* Period selector */}
      <div className="flex items-center gap-2">
        <Clock className="h-4 w-4 text-gray-500" />
        <div className="flex rounded-md shadow-sm">
          {periods.map((p) => (
            <button
              key={p.value}
              onClick={() => onPeriodChange(p.value)}
              className={`
                px-3 py-1 text-sm font-medium
                ${period === p.value 
                  ? 'bg-indigo-600 text-white' 
                  : 'bg-white text-gray-700 hover:bg-gray-50'
                }
                ${p.default ? 'rounded-l-md' : ''}
                ${p.value === '24h' ? 'rounded-r-md' : ''}
                border border-gray-300
              `}
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      {/* Granularity selector */}
      <div className="flex items-center gap-2">
        <Activity className="h-4 w-4 text-gray-500" />
        <select
          value={granularity}
          onChange={(e) => onGranularityChange(e.target.value)}
          className="block w-32 rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
        >
          {availableGranularities.map((g) => (
            <option key={g.value} value={g.value}>
              {g.label}
            </option>
          ))}
        </select>
      </div>

      {/* Reset button - visible only when exploring */}
      {isExploring && (
        <button
          onClick={onReset}
          className="inline-flex items-center px-3 py-1 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
        >
          <RotateCcw className="h-4 w-4 mr-1" />
          Reset to default
        </button>
      )}

      {/* Indicator */}
      <div className="text-sm text-gray-500">
        {isExploring ? (
          <span className="text-amber-600">
            Exploring mode - higher bandwidth usage
          </span>
        ) : (
          <span className="text-green-600">
            Default view - optimized performance
          </span>
        )}
      </div>
    </div>
  );
};

export default MessageRateControls;
