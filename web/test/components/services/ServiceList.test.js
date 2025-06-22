import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import ServiceList from '../../../src/components/services/ServiceList';

describe('ServiceList', () => {
  const mockServices = [
    {
      id: 'service-1',
      name: 'web-frontend',
      enabled: true,
      permissions: ['publish:orders', 'consume:notifications'],
      lastUsed: '2024-01-01T10:00:00Z'
    },
    {
      id: 'service-2',
      name: 'analytics-processor',
      enabled: false,
      permissions: ['consume:*'],
      lastUsed: '0001-01-01T00:00:00Z' // Never used
    }
  ];

  const defaultProps = {
    services: mockServices,
    loading: false,
    editingService: null,
    onEdit: jest.fn(),
    onSave: jest.fn(),
    onCancel: jest.fn(),
    onRotateSecret: jest.fn(),
    onDelete: jest.fn(),
    formatDate: (date) => date === '0001-01-01T00:00:00Z' ? 'Never' : '01/01/2024'
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders loading state', () => {
    render(<ServiceList {...defaultProps} loading={true} />);

    expect(screen.getByText(/loading services/i)).toBeInTheDocument();
  });

  test('renders empty state', () => {
    render(<ServiceList {...defaultProps} services={[]} />);

    expect(screen.getByText(/no service accounts yet/i)).toBeInTheDocument();
    expect(screen.getByText(/create your first service/i)).toBeInTheDocument();
  });

  test('renders services list', () => {
    render(<ServiceList {...defaultProps} />);

    // Test service names
    expect(screen.getByText('web-frontend')).toBeInTheDocument();
    expect(screen.getByText('analytics-processor')).toBeInTheDocument();
    
    expect(screen.getByText((content, element) => {
      return element?.textContent === 'ID: service-1';
    })).toBeInTheDocument();
    
    expect(screen.getByText((content, element) => {
      return element?.textContent === 'ID: service-2';
    })).toBeInTheDocument();
  });

  test('displays correct status badges', () => {
    render(<ServiceList {...defaultProps} />);

    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();
  });

  test('shows permissions with overflow indicator', () => {
    const serviceWithManyPermissions = {
      ...mockServices[0],
      permissions: ['publish:orders', 'consume:notifications', 'manage:analytics']
    };

    render(
      <ServiceList 
        {...defaultProps} 
        services={[serviceWithManyPermissions]} 
      />
    );

    expect(screen.getByText('publish:orders')).toBeInTheDocument();
    expect(screen.getByText('consume:notifications')).toBeInTheDocument();
    expect(screen.getByText('+1 more')).toBeInTheDocument();
  });

  test('calls onEdit when edit button clicked', () => {
    const onEdit = jest.fn();

    render(<ServiceList {...defaultProps} onEdit={onEdit} />);

    const editButtons = screen.getAllByTitle('Edit permissions');
    fireEvent.click(editButtons[0]);

    expect(onEdit).toHaveBeenCalledWith(mockServices[0]);
  });

  test('calls onRotateSecret when rotate button clicked', () => {
    const onRotateSecret = jest.fn();

    render(<ServiceList {...defaultProps} onRotateSecret={onRotateSecret} />);

    const rotateButtons = screen.getAllByTitle('Rotate secret');
    fireEvent.click(rotateButtons[0]);

    expect(onRotateSecret).toHaveBeenCalledWith('service-1');
  });

  test('calls onDelete when delete button clicked', () => {
    const onDelete = jest.fn();

    render(<ServiceList {...defaultProps} onDelete={onDelete} />);

    const deleteButtons = screen.getAllByTitle('Delete service');
    fireEvent.click(deleteButtons[0]);

    expect(onDelete).toHaveBeenCalledWith('service-1', 'web-frontend');
  });

  test('shows save/cancel buttons when editing', () => {
    render(<ServiceList {...defaultProps} editingService="service-1" />);

    expect(screen.getByTitle('Save changes')).toBeInTheDocument();
    expect(screen.getByTitle('Cancel')).toBeInTheDocument();
    
    // Edit/rotate/delete buttons should not be visible for editing service
    const editButtons = screen.queryAllByTitle('Edit permissions');
    expect(editButtons).toHaveLength(mockServices.length - 1);
  });

  test('calls onSave when save button clicked', () => {
    const onSave = jest.fn();

    render(<ServiceList {...defaultProps} editingService="service-1" onSave={onSave} />);

    fireEvent.click(screen.getByTitle('Save changes'));

    expect(onSave).toHaveBeenCalled();
  });

  test('calls onCancel when cancel button clicked', () => {
    const onCancel = jest.fn();

    render(<ServiceList {...defaultProps} editingService="service-1" onCancel={onCancel} />);

    fireEvent.click(screen.getByTitle('Cancel'));

    expect(onCancel).toHaveBeenCalled();
  });

  test('formats dates correctly', () => {
    render(<ServiceList {...defaultProps} />);

    expect(screen.getByText('01/01/2024')).toBeInTheDocument();
    expect(screen.getByText('Never')).toBeInTheDocument();
  });

  test('displays service count in header', () => {
    render(<ServiceList {...defaultProps} />);

    expect(screen.getByText('2 total services')).toBeInTheDocument();
  });
});
