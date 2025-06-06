import React, { useState } from 'react';
import { Menu, Bell, Search, User, Settings, LogOut } from 'lucide-react';
import { useNotifications } from '../../hooks/useNotifications';

export const Header = ({ toggleSidebar }) => {
  const [profileOpen, setProfileOpen] = useState(false);
  const { notifications, unreadCount, markAsRead } = useNotifications();

  return (
    <header className="bg-white border-b border-gray-200">
      <div className="px-4 sm:px-6 lg:px-8 flex justify-between h-16">
        <div className="flex items-center">
          <button
            className="p-2 rounded-md text-gray-500 lg:hidden"
            onClick={toggleSidebar}
            aria-label="menu"
          >
            <Menu className="h-6 w-6" />
          </button>
          <div className="ml-4 lg:ml-0 flex items-center">
            <span className="text-lg font-bold text-indigo-600">GoRTMS</span>
          </div>
        </div>

        <div className="flex items-center space-x-4">
          <SearchBar />
          <NotificationDropdown
            notifications={notifications}
            unreadCount={unreadCount}
            onMarkAsRead={markAsRead}
          />
          <ProfileDropdown
            isOpen={profileOpen}
            setIsOpen={setProfileOpen}
          />
        </div>
      </div>
    </header>
  );
};

const SearchBar = () => (
  <div className="hidden sm:block relative">
    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
      <Search className="h-5 w-5 text-gray-400" />
    </div>
    <input
      type="text"
      placeholder="Search..."
      className="pl-10 w-64 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-indigo-500 focus:border-indigo-500"
    />
  </div>
);

const NotificationDropdown = ({ notifications, unreadCount, onMarkAsRead }) => {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <div className="relative">
      <button
        className="p-2 rounded-full text-gray-500 hover:bg-gray-100 relative"
        onClick={() => setIsOpen(!isOpen)}
      >
        <Bell className="h-6 w-6" />
        {unreadCount > 0 && (
          <span className="absolute top-0 right-0 block h-2 w-2 rounded-full bg-red-500"></span>
        )}
      </button>

      {isOpen && (
        <div className="absolute right-0 mt-2 w-80 bg-white rounded-md shadow-lg overflow-hidden z-10">
          <div className="px-4 py-2 border-b border-gray-200">
            <h3 className="text-sm font-medium text-gray-700">Notifications</h3>
          </div>
          <div className="divide-y divide-gray-200 max-h-96 overflow-y-auto">
            {notifications.map((notification) => (
              <NotificationItem
                key={notification.id}
                notification={notification}
                onRead={() => onMarkAsRead(notification.id)}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

const ProfileDropdown = ({ isOpen, setIsOpen }) => (
  <div className="relative">
    <button
      className="flex items-center space-x-2 text-gray-700 hover:text-gray-900"
      onClick={() => setIsOpen(!isOpen)}
    >
      <div className="h-8 w-8 rounded-full bg-indigo-200 flex items-center justify-center">
        <User className="h-5 w-5 text-indigo-600" />
      </div>
      <span className="hidden md:block text-sm font-medium">Admin User</span>
    </button>

    {isOpen && (
      <div className="absolute right-0 mt-2 w-48 bg-white rounded-md shadow-lg overflow-hidden z-10">
        <div className="py-1">
          <button className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 w-full text-left">
            <User className="h-4 w-4 mr-2" />
            Your Profile
          </button>
          <button className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 w-full text-left">
            <Settings className="h-4 w-4 mr-2" />
            Settings
          </button>
          <button className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 w-full text-left">
            <LogOut className="h-4 w-4 mr-2" />
            Sign out
          </button>
        </div>
      </div>
    )}
  </div>
);
