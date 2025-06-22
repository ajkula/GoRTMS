import React from 'react';
import { X } from 'lucide-react';

const IPWhitelistManager = ({ 
  ipWhitelist,
  ipInput,
  setIpInput,
  onAdd,
  onRemove
}) => {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-3">
        IP Whitelist (Optional)
      </label>
      
      <div className="space-y-3">
        <div className="flex space-x-2">
          <input
            type="text"
            value={ipInput}
            onChange={(e) => setIpInput(e.target.value)}
            placeholder="IP address (e.g., 192.168.1.10, 10.0.*, *)"
            className="flex-1 px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            onKeyPress={(e) => e.key === 'Enter' && onAdd()}
          />
          <button
            type="button"
            onClick={onAdd}
            className="px-4 py-2 text-sm bg-green-600 text-white rounded-md hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-green-500 transition-colors"
          >
            Add IP
          </button>
        </div>
        
        <div className="space-y-2">
          {ipWhitelist.map((ip, index) => (
            <div key={index} className="flex items-center justify-between bg-green-50 px-3 py-2 rounded-md">
              <span className="text-sm font-mono text-green-800">{ip}</span>
              <button
                type="button"
                onClick={() => onRemove(ip)}
                className="text-red-600 hover:text-red-800 transition-colors"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
        
        {ipWhitelist.length === 0 && (
          <p className="text-sm text-gray-500 italic">No IP restrictions (allows all IPs)</p>
        )}
      </div>
    </div>
  );
};

export default IPWhitelistManager;
