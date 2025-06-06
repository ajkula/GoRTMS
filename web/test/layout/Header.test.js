import React from 'react';
import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Header } from '../../src/components/layout/Header';
import { useHealthCheck } from '../../src/hooks/useHealthCheck';

// Mock the health check hook
jest.mock('../../src/hooks/useHealthCheck');

describe('Header', () => {
  const mockToggleSidebar = jest.fn();
  
  const defaultProps = {
    isSidebarOpen: true,
  toggleSidebar: mockToggleSidebar,
  };

  beforeEach(() => {
    jest.clearAllMocks();
    useHealthCheck.mockReturnValue({
      status: 'healthy',
      lastCheck: new Date()
    });
  });

  test('renders header with title', async () => {
    await act(() => render(<Header {...defaultProps} />));
    
    expect(screen.getByText('GoRTMS')).toBeInTheDocument();
  });

  test('toggles sidebar when menu button clicked', async () => {
    const user = userEvent.setup();
    await act(() => render(<Header {...defaultProps} />));
    
    const menuButton = screen.getByRole('button', { name: /menu/i });
    await act(() => user.click(menuButton));

    expect(mockToggleSidebar).toHaveBeenCalled();
  });

  // test.only('displays health status indicator', async () => {
  //   await act(() => render(<Header {...defaultProps} />));
    
  //   // Should show healthy indicator (green)
  //   const healthIndicator = screen.getByTestId('health-indicator');
  //   expect(healthIndicator).toHaveClass('bg-green-500');
  // });

  // test.only('shows different colors for health status', () => {
  //   const { rerender } = render(<Header {...defaultProps} />);
    
  //   // Unhealthy status
  //   useHealthCheck.mockReturnValue({
  //     status: 'unhealthy',
  //     lastCheck: new Date()
  //   });
    
  //   rerender(<Header {...defaultProps} />);
    
  //   const healthIndicator = screen.getByTestId('health-indicator');
  //   expect(healthIndicator).toHaveClass('bg-red-500');
  // });

  // test.only('displays last check time in tooltip', async () => {
  //   const user = userEvent.setup();
  //   const checkTime = new Date('2025-01-15T10:30:00');
    
  //   useHealthCheck.mockReturnValue({
  //     status: 'healthy',
  //     lastCheck: checkTime
  //   });
    
  //   await act(() => render(<Header {...defaultProps} />));
    
  //   const healthIndicator = screen.getByTestId('health-indicator');
  //   await user.hover(healthIndicator);
    
  //   // Should show formatted time in tooltip
  //   await screen.findByText(/Last check:/);
  // });
});
