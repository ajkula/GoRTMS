import React from 'react';
import {
  X, LayoutDashboard, Database, MessageSquare, Shield,
  GitBranch, User, Settings
} from 'lucide-react';
import { useAuth } from '../../hooks/useAuth';

export const Sidebar = ({ isOpen, toggleSidebar, navigate, currentPage, setPage }) => {
  const { isAdmin } = useAuth();
  const isActive = (item) => {
    if (currentPage.type === item.type) return true;
    if (item.alsoActive && item.alsoActive.includes(currentPage.type)) return true;
    return false;
  };

  const menuItems = [
    { type: 'dashboard', icon: LayoutDashboard, label: 'Dashboard', admin: false },
    { type: 'domains', icon: Database, label: 'Domains', admin: false },
    { type: 'queues', icon: MessageSquare, label: 'Queues', alsoActive: ['queue-monitor'], admin: false },
    { type: 'services', icon: Shield, label: 'Service Accounts', admin: false },
    { type: 'routes', icon: GitBranch, label: 'Routing', admin: false },
    { type: 'consumer-groups', icon: User, label: 'Consumer Groups', alsoActive: ['consumer-group-detail'], admin: false },
    { type: 'settings', icon: Settings, label: 'Settings', admin: true },
  ].filter(button => !button.admin || isAdmin);


  return (
    <>
      {/* Mobile overlay */}
      {isOpen && (
        <div
          className="fixed inset-0 bg-gray-600 bg-opacity-75 z-20 lg:hidden"
          onClick={toggleSidebar}
        />
      )}

      {/* Sidebar */}
      <div className={`
        fixed inset-y-0 left-0 w-64 bg-white border-r border-gray-200 z-30
        transform transition-transform duration-300 ease-in-out
        ${isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
      `}>
        <div className="h-16 flex items-center justify-between px-4 border-b border-gray-200 lg:hidden">
          <span className="text-lg font-bold text-indigo-600">GoRTMS</span>
          <button className="p-2 rounded-md text-gray-500" onClick={toggleSidebar}>
            <X className="h-6 w-6" />
          </button>
        </div>

        <div className="overflow-y-auto h-full pb-8">
          <nav className="px-2 py-4 space-y-1">
            {menuItems.map((item) => {
              const Icon = item.icon;
              const active = isActive(item);

              return (
                <button
                  key={item.type}
                  onClick={() => {
                    if (item.type === 'queues' && !currentPage.domainName) {
                      navigate.toDomains();
                    } else {
                      setPage({ type: item.type });
                    }
                  }}
                  className={`flex items-center px-4 py-2 text-base font-medium rounded-md w-full text-left
                    ${active ? 'bg-indigo-50 text-indigo-700' : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}
                  `}
                >
                  <Icon className="mr-3 h-5 w-5" />
                  {item.label}
                </button>
              );
            })}
          </nav>

          <HelpSection />
        </div>
      </div>
    </>
  );
};

const HelpSection = () => (
  <div className="px-4 py-4 mt-6">
    <div className="bg-indigo-50 rounded-lg px-4 py-3">
      <h3 className="text-sm font-medium text-indigo-800 mb-1">Need help?</h3>
      <p className="text-xs text-indigo-600">
        Check out our documentation or contact support for assistance.
      </p>
      <button className="mt-2 text-xs font-medium text-indigo-700 hover:text-indigo-900 block">
        View Documentation →
      </button>
    </div>
  </div>
);
