import React from 'react';
import { Activity, MessageSquare, Database, GitBranch, ArrowUpRight, ArrowDownRight } from 'lucide-react';

const StatCard = ({ title, value, icon, trend, trendValue }) => {
  const Icon = icon;
  const isTrendUp = trend === 'up';

  return (
    <div className="bg-white rounded-lg shadow p-6 flex flex-col">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-gray-500 text-sm font-medium">{title}</h3>
        <div className="bg-indigo-100 p-2 rounded-lg">
          <Icon className="h-5 w-5 text-indigo-600" />
        </div>
      </div>
      <div className="flex items-baseline">
        <span className="text-3xl font-bold text-gray-900">{value !== undefined ? value.toLocaleString() : '-'}</span>
        {trendValue ? (
          <span className={`ml-2 flex items-center text-sm ${isTrendUp ? 'text-green-500' : 'text-red-500'}`}>
            {isTrendUp ? <ArrowUpRight size={16} /> : <ArrowDownRight size={16} />}
            {Math.floor(trendValue)}%
          </span>
        ) : (
          <span className="ml-2 flex items-center text-sm">0</span>
        )}
      </div>
    </div>
  );
};

const StatCards = ({ stats }) => {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      <StatCard
        title="Total Domains"
        value={stats?.domains}
        icon={Database}
        trend={stats?.domainTrend?.direction}
        trendValue={stats?.domainTrend?.value}
      />
      <StatCard
        title="Total Queues"
        value={stats?.queues}
        icon={MessageSquare}
        trend={stats?.queueTrend?.direction}
        trendValue={stats?.queueTrend?.value}
      />
      <StatCard
        title="Total Messages"
        value={stats?.messages}
        icon={Activity}
        trend={stats?.messageTrend?.direction}
        trendValue={stats?.messageTrend?.value}
      />
      <StatCard
        title="Routing Rules"
        value={stats?.routes}
        icon={GitBranch}
        trend={stats?.routeTrend?.direction}
        trendValue={stats?.routeTrend?.value}
      />
    </div>
  );
};

export default StatCards;
