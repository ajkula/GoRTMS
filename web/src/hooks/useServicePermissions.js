import { useState, useCallback } from 'react';

export const useServicePermissions = (initialPermissions = [], initialIpWhitelist = []) => {
  const [permissions, setPermissions] = useState(initialPermissions);
  const [ipWhitelist, setIpWhitelist] = useState(initialIpWhitelist);
  const [permissionBuilder, setPermissionBuilder] = useState({
    action: 'publish',
    domain: '*'
  });
  const [ipInput, setIpInput] = useState('');

  const addPermission = useCallback(() => {
    const { action, domain } = permissionBuilder;
    const permission = action === '*' ? '*' : `${action}:${domain}`;
    
    if (!permissions.includes(permission)) {
      setPermissions(prev => [...prev, permission]);
    }
  }, [permissionBuilder, permissions]);

  const removePermission = useCallback((permission) => {
    setPermissions(prev => prev.filter(p => p !== permission));
  }, []);

  const addIP = useCallback(() => {
    const ip = ipInput.trim();
    if (!ip || ipWhitelist.includes(ip)) return;
    
    setIpWhitelist(prev => [...prev, ip]);
    setIpInput('');
  }, [ipInput, ipWhitelist]);

  const removeIP = useCallback((ip) => {
    setIpWhitelist(prev => prev.filter(i => i !== ip));
  }, []);

  const resetPermissions = useCallback((newPermissions = [], newIpWhitelist = []) => {
    setPermissions(newPermissions);
    setIpWhitelist(newIpWhitelist);
  }, []);

  return {
    permissions,
    ipWhitelist,
    permissionBuilder,
    setPermissionBuilder,
    ipInput,
    setIpInput,
    addPermission,
    removePermission,
    addIP,
    removeIP,
    resetPermissions
  };
};
