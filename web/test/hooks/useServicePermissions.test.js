import { renderHook, act } from '@testing-library/react';
import { useServicePermissions } from '../../src/hooks/useServicePermissions';

describe('useServicePermissions', () => {
  test('initializes with default values', () => {
    const { result } = renderHook(() => useServicePermissions());

    expect(result.current.permissions).toEqual([]);
    expect(result.current.ipWhitelist).toEqual([]);
    expect(result.current.permissionBuilder).toEqual({
      action: 'publish',
      domain: '*'
    });
    expect(result.current.ipInput).toBe('');
  });

  test('initializes with provided values', () => {
    const initialPermissions = ['publish:orders', 'consume:*'];
    const initialIPs = ['192.168.1.10'];

    const { result } = renderHook(() => 
      useServicePermissions(initialPermissions, initialIPs)
    );

    expect(result.current.permissions).toEqual(initialPermissions);
    expect(result.current.ipWhitelist).toEqual(initialIPs);
  });

  test('adds permission correctly', () => {
    const { result } = renderHook(() => useServicePermissions());

    act(() => {
      result.current.setPermissionBuilder({
        action: 'publish',
        domain: 'orders'
      });
    });

    act(() => {
      result.current.addPermission();
    });

    expect(result.current.permissions).toEqual(['publish:orders']);
  });

  test('adds wildcard permission', () => {
    const { result } = renderHook(() => useServicePermissions());

    act(() => {
      result.current.setPermissionBuilder({
        action: '*',
        domain: 'ignored'
      });
    });

    act(() => {
      result.current.addPermission();
    });

    expect(result.current.permissions).toEqual(['*']);
  });

  test('prevents duplicate permissions', () => {
    const { result } = renderHook(() => useServicePermissions(['publish:orders']));

    act(() => {
      result.current.setPermissionBuilder({
        action: 'publish',
        domain: 'orders'
      });
    });

    act(() => {
      result.current.addPermission();
    });

    expect(result.current.permissions).toEqual(['publish:orders']);
  });

  test('removes permission', () => {
    const { result } = renderHook(() => 
      useServicePermissions(['publish:orders', 'consume:*'])
    );

    act(() => {
      result.current.removePermission('publish:orders');
    });

    expect(result.current.permissions).toEqual(['consume:*']);
  });

  test('adds IP address', () => {
    const { result } = renderHook(() => useServicePermissions());

    act(() => {
      result.current.setIpInput('192.168.1.10');
    });

    act(() => {
      result.current.addIP();
    });

    expect(result.current.ipWhitelist).toEqual(['192.168.1.10']);
    expect(result.current.ipInput).toBe('');
  });

  test('prevents duplicate IP addresses', () => {
    const { result } = renderHook(() => useServicePermissions([], ['192.168.1.10']));

    act(() => {
      result.current.setIpInput('192.168.1.10');
    });

    act(() => {
      result.current.addIP();
    });

    expect(result.current.ipWhitelist).toEqual(['192.168.1.10']);
  });

  test('ignores empty IP input', () => {
    const { result } = renderHook(() => useServicePermissions());

    act(() => {
      result.current.setIpInput('   ');
    });

    act(() => {
      result.current.addIP();
    });

    expect(result.current.ipWhitelist).toEqual([]);
  });

  test('removes IP address', () => {
    const { result } = renderHook(() => 
      useServicePermissions([], ['192.168.1.10', '10.0.0.1'])
    );

    act(() => {
      result.current.removeIP('192.168.1.10');
    });

    expect(result.current.ipWhitelist).toEqual(['10.0.0.1']);
  });

  test('resets permissions and IPs', () => {
    const { result } = renderHook(() => 
      useServicePermissions(['publish:orders'], ['192.168.1.10'])
    );

    const newPermissions = ['consume:*'];
    const newIPs = ['10.0.0.1'];

    act(() => {
      result.current.resetPermissions(newPermissions, newIPs);
    });

    expect(result.current.permissions).toEqual(newPermissions);
    expect(result.current.ipWhitelist).toEqual(newIPs);
  });
});
