import React from 'react';
import { AlertTriangle, CheckCircle } from 'lucide-react';

const StatusMessages = ({ error, success, onClearError, onClearSuccess }) => (
  <>
    {error && (
      <div className="mb-4 bg-red-50 border border-red-200 rounded-md p-4">
        <div className="flex">
          <AlertTriangle className="h-5 w-5 text-red-400" />
          <div className="ml-3 flex-1">
            <h3 className="text-sm font-medium text-red-800">Error</h3>
            <p className="text-sm text-red-700 mt-1">{error}</p>
          </div>
          <button
            onClick={onClearError}
            className="ml-3 text-red-400 hover:text-red-600"
          >
            ×
          </button>
        </div>
      </div>
    )}

    {success && (
      <div className="mb-4 bg-green-50 border border-green-200 rounded-md p-4">
        <div className="flex">
          <CheckCircle className="h-5 w-5 text-green-400" />
          <div className="ml-3 flex-1">
            <h3 className="text-sm font-medium text-green-800">Success</h3>
            <p className="text-sm text-green-700 mt-1">{success}</p>
          </div>
          <button
            onClick={onClearSuccess}
            className="ml-3 text-green-400 hover:text-green-600"
          >
            ×
          </button>
        </div>
      </div>
    )}
  </>
);

export default StatusMessages;
