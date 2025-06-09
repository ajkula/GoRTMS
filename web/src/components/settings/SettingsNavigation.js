import React from 'react';
import { Settings, Server, Shield, Database, Globe } from 'lucide-react';

const SettingsNavigation = ({ activeSection, onSectionChange }) => {
  const sections = [
    { id: 'general', label: 'General', icon: Settings },
    { id: 'http', label: 'HTTP Server', icon: Globe },
    { id: 'security', label: 'Security', icon: Shield },
    { id: 'storage', label: 'Storage', icon: Database },
    { id: 'grpc', label: 'gRPC', icon: Server },
  ];

  return (
    <div className="w-64 bg-white rounded-lg shadow-sm p-4">
      <nav className="space-y-1">
        {sections.map((section) => {
          const Icon = section.icon;
          return (
            <button
              key={section.id}
              onClick={() => onSectionChange(section.id)}
              className={`w-full flex items-center px-3 py-2 text-sm font-medium rounded-md ${
                activeSection === section.id
                  ? 'bg-indigo-100 text-indigo-700'
                  : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
              }`}
            >
              <Icon className="h-4 w-4 mr-3" />
              {section.label}
            </button>
          );
        })}
      </nav>
    </div>
  );
};

export default SettingsNavigation;
