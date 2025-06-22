import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ServiceCreateForm from '../../../src/components/services/ServiceCreateForm';
import { useServicePermissions } from '../../../src/hooks/useServicePermissions';

jest.mock('../../../src/hooks/useServicePermissions');

jest.mock('../../../src/components/services/PermissionBuilder', () => {
  return function MockPermissionBuilder({ permissions, onAdd, onRemove }) {
    return (
      <div data-testid="permission-builder">
        <div>Permissions: {permissions.join(', ')}</div>
        <button onClick={onAdd}>Add Permission</button>
        {permissions.map((perm, index) => (
          <button key={index} onClick={() => onRemove(perm)}>Remove {perm}</button>
        ))}
      </div>
    );
  };
});

jest.mock('../../../src/components/services/IPWhitelistManager', () => {
  return function MockIPWhitelistManager({ ipWhitelist, onAdd, onRemove }) {
    return (
      <div data-testid="ip-whitelist-manager">
        <div>IPs: {ipWhitelist.join(', ')}</div>
        <button onClick={onAdd}>Add IP</button>
        {ipWhitelist.map((ip, index) => (
          <button key={index} onClick={() => onRemove(ip)}>Remove {ip}</button>
        ))}
      </div>
    );
  };
});

describe('ServiceCreateForm', () => {
  const mockUseServicePermissions = {
    permissions: [],
    ipWhitelist: [],
    permissionBuilder: { action: 'publish', domain: '*' },
    setPermissionBuilder: jest.fn(),
    ipInput: '',
    setIpInput: jest.fn(),
    addPermission: jest.fn(),
    removePermission: jest.fn(),
    addIP: jest.fn(),
    removeIP: jest.fn()
  };

  const defaultProps = {
    domains: [{ name: 'orders' }, { name: 'analytics' }],
    onSubmit: jest.fn(),
    onCancel: jest.fn(),
    loading: false
  };

  beforeEach(() => {
    jest.clearAllMocks();
    
    global.alert = jest.fn();
    
    useServicePermissions.mockReturnValue(mockUseServicePermissions);
  });

  afterEach(() => {
    global.alert.mockRestore?.();
  });

  test('renders form elements', () => {
    render(<ServiceCreateForm {...defaultProps} />);

    expect(screen.getByLabelText(/service name/i)).toBeInTheDocument();
    expect(screen.getByTestId('permission-builder')).toBeInTheDocument();
    expect(screen.getByTestId('ip-whitelist-manager')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create service/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
  });

  test('validates required fields', async () => {
    const onSubmit = jest.fn();

    render(<ServiceCreateForm {...defaultProps} onSubmit={onSubmit} />);

    const form = document.querySelector('form');
    const nameInput = screen.getByLabelText(/service name/i);
    
    nameInput.removeAttribute('required');
    
    fireEvent.submit(form);

    await waitFor(() => {
      expect(global.alert).toHaveBeenCalledWith('Service name is required');
    });
    
    expect(onSubmit).not.toHaveBeenCalled();
  });

  test('validates required fields via button click', async () => {
    const onSubmit = jest.fn();

    render(<ServiceCreateForm {...defaultProps} onSubmit={onSubmit} />);

    const nameInput = screen.getByLabelText(/service name/i);
    const submitButton = screen.getByRole('button', { name: /create service/i });
    
    nameInput.removeAttribute('required');
    
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(global.alert).toHaveBeenCalledWith('Service name is required');
    });
    
    expect(onSubmit).not.toHaveBeenCalled();
  });

  test('validates permissions requirement', async () => {
    render(<ServiceCreateForm {...defaultProps} />);

    const nameInput = screen.getByLabelText(/service name/i);
    nameInput.removeAttribute('required');
    
    fireEvent.change(nameInput, { target: { value: 'test-service' } });

    const submitButton = screen.getByRole('button', { name: /create service/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(global.alert).toHaveBeenCalledWith('At least one permission is required');
    });
  });

  test('submits form with valid data', async () => {
    const onSubmit = jest.fn();
    const mockPermissions = ['publish:orders'];
    const mockIpWhitelist = ['192.168.1.10'];

    useServicePermissions.mockReturnValue({
      ...mockUseServicePermissions,
      permissions: mockPermissions,
      ipWhitelist: mockIpWhitelist
    });

    render(<ServiceCreateForm {...defaultProps} onSubmit={onSubmit} />);

    const nameInput = screen.getByLabelText(/service name/i);
    fireEvent.change(nameInput, { target: { value: 'test-service' } });

    const submitButton = screen.getByRole('button', { name: /create service/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        name: 'test-service',
        permissions: mockPermissions,
        ipWhitelist: mockIpWhitelist
      });
    });

    expect(global.alert).not.toHaveBeenCalled();
  });

  test('shows loading state', () => {
    render(<ServiceCreateForm {...defaultProps} loading={true} />);

    const submitButton = screen.getByRole('button', { name: /creating/i });
    expect(submitButton).toBeDisabled();
  });

  test('calls onCancel when cancel button clicked', () => {
    const onCancel = jest.fn();

    render(<ServiceCreateForm {...defaultProps} onCancel={onCancel} />);

    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));

    expect(onCancel).toHaveBeenCalled();
  });

  test('trims whitespace from service name', async () => {
    const onSubmit = jest.fn();
    const mockPermissions = ['publish:orders'];

    useServicePermissions.mockReturnValue({
      ...mockUseServicePermissions,
      permissions: mockPermissions
    });

    render(<ServiceCreateForm {...defaultProps} onSubmit={onSubmit} />);

    const nameInput = screen.getByLabelText(/service name/i);
    fireEvent.change(nameInput, { target: { value: '  test-service  ' } });

    const submitButton = screen.getByRole('button', { name: /create service/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        name: 'test-service',
        permissions: mockPermissions,
        ipWhitelist: []
      });
    });
  });

  test('calls useServicePermissions hook on mount', () => {
    render(<ServiceCreateForm {...defaultProps} />);

    expect(useServicePermissions).toHaveBeenCalled();
  });

  test('handles empty name with only spaces', async () => {
    const onSubmit = jest.fn();

    render(<ServiceCreateForm {...defaultProps} onSubmit={onSubmit} />);

    const nameInput = screen.getByLabelText(/service name/i);
    nameInput.removeAttribute('required');
    
    fireEvent.change(nameInput, { target: { value: '   ' } });

    const submitButton = screen.getByRole('button', { name: /create service/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(global.alert).toHaveBeenCalledWith('Service name is required');
    });
    
    expect(onSubmit).not.toHaveBeenCalled();
  });

  test('handles form submission with HTML5 validation disabled', async () => {
    const onSubmit = jest.fn();

    const { container } = render(<ServiceCreateForm {...defaultProps} onSubmit={onSubmit} />);
    
    const form = container.querySelector('form');
    if (form) {
      form.setAttribute('novalidate', 'true');
    }

    const submitButton = screen.getByRole('button', { name: /create service/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(global.alert).toHaveBeenCalledWith('Service name is required');
    });
    
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
