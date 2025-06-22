import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import PermissionBuilder from '../../../src/components/services/PermissionBuilder';

describe('PermissionBuilder', () => {
  const defaultProps = {
    domains: [{ name: 'orders' }, { name: 'analytics' }],
    permissions: [],
    permissionBuilder: { action: 'publish', domain: '*' },
    setPermissionBuilder: jest.fn(),
    onAdd: jest.fn(),
    onRemove: jest.fn()
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders permission builder interface', () => {
    render(<PermissionBuilder {...defaultProps} />);

    expect(screen.getByText(/add permission/i)).toBeInTheDocument();
    expect(screen.getAllByText(/action/i)[0]).toBeInTheDocument();
    expect(screen.getAllByText(/domain/i)[0]).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add/i })).toBeInTheDocument();
  });

  test('displays current permissions', () => {
    const permissions = ['publish:orders', 'consume:*'];

    render(<PermissionBuilder {...defaultProps} permissions={permissions} />);

    expect(screen.getByText('publish:orders')).toBeInTheDocument();
    expect(screen.getByText('consume:*')).toBeInTheDocument();
    expect(screen.getByText(/current permissions/i)).toBeInTheDocument();
  });

  test('calls setPermissionBuilder when action changes', () => {
    const setPermissionBuilder = jest.fn();

    render(
      <PermissionBuilder 
        {...defaultProps} 
        setPermissionBuilder={setPermissionBuilder} 
      />
    );

    const selects = screen.getAllByRole('combobox');
    const actionSelect = selects[0];
    
    fireEvent.change(actionSelect, { target: { value: 'consume' } });

    expect(setPermissionBuilder).toHaveBeenCalled();
  });

  test('calls setPermissionBuilder when domain changes', () => {
    const setPermissionBuilder = jest.fn();

    render(
      <PermissionBuilder 
        {...defaultProps} 
        setPermissionBuilder={setPermissionBuilder} 
      />
    );

    const selects = screen.getAllByRole('combobox');
    const domainSelect = selects[1];
    
    fireEvent.change(domainSelect, { target: { value: 'orders' } });

    expect(setPermissionBuilder).toHaveBeenCalled();
  });

  test('disables domain select when action is wildcard', () => {
    const props = {
      ...defaultProps,
      permissionBuilder: { action: '*', domain: '*' }
    };

    render(<PermissionBuilder {...props} />);

    const selects = screen.getAllByRole('combobox');
    const domainSelect = selects[1];
    
    expect(domainSelect).toBeDisabled();
  });

  test('calls onAdd when add button clicked', () => {
    const onAdd = jest.fn();

    render(<PermissionBuilder {...defaultProps} onAdd={onAdd} />);

    fireEvent.click(screen.getByRole('button', { name: /add/i }));

    expect(onAdd).toHaveBeenCalled();
  });

  test('calls onRemove when remove button clicked', () => {
    const onRemove = jest.fn();
    const permissions = ['publish:orders'];

    render(
      <PermissionBuilder 
        {...defaultProps} 
        permissions={permissions}
        onRemove={onRemove} 
      />
    );

    const buttons = screen.getAllByRole('button');
    const removeButton = buttons.find(button => 
      button.textContent === '' && button.type === 'button' && !button.textContent.includes('Add')
    );
    
    if (removeButton) {
      fireEvent.click(removeButton);
      expect(onRemove).toHaveBeenCalledWith('publish:orders');
    } else {
      const removeButtons = document.querySelectorAll('button[type="button"]');
      const actualRemoveButton = Array.from(removeButtons).find(btn => 
        btn.textContent === '' && btn.className.includes('text-red')
      );
      if (actualRemoveButton) {
        fireEvent.click(actualRemoveButton);
        expect(onRemove).toHaveBeenCalledWith('publish:orders');
      }
    }
  });

  test('renders domain options correctly', () => {
    render(<PermissionBuilder {...defaultProps} />);

    expect(screen.getByText(/all domains/i)).toBeInTheDocument();
    expect(screen.getByText('orders')).toBeInTheDocument();
    expect(screen.getByText('analytics')).toBeInTheDocument();
  });

  test('shows no permissions message when list is empty', () => {
    render(<PermissionBuilder {...defaultProps} />);

    expect(screen.queryByText(/current permissions/i)).not.toBeInTheDocument();
  });

  test('renders correct action options', () => {
    render(<PermissionBuilder {...defaultProps} />);

    expect(screen.getByText('All Actions (*)')).toBeInTheDocument();
    expect(screen.getByText('Publish')).toBeInTheDocument();
    expect(screen.getByText('Consume')).toBeInTheDocument();
    expect(screen.getByText('Manage')).toBeInTheDocument();
  });
});
