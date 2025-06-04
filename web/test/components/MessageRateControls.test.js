import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MessageRateControls from '../../src/components/MessageRateControls';

describe('MessageRateControls', () => {
  const defaultProps = {
    period: '1h',
    granularity: 'auto',
    isExploring: false,
    periods: [
      { value: '1h', label: '1 Hour', default: true },
      { value: '6h', label: '6 Hours' },
    ],
    availableGranularities: [
      { value: 'auto', label: 'Auto (1m)', default: true },
      { value: '10s', label: '10 seconds' },
    ],
    onPeriodChange: jest.fn(),
    onGranularityChange: jest.fn(),
    onReset: jest.fn(),
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders all controls', () => {
    render(<MessageRateControls {...defaultProps} />);

    expect(screen.getByText('1 Hour')).toBeInTheDocument();
    expect(screen.getByText('6 Hours')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
    expect(screen.getByText(/Default view/)).toBeInTheDocument();
  });

  test('calls onPeriodChange when period button clicked', async () => {
    const user = userEvent.setup();
    render(<MessageRateControls {...defaultProps} />);

    await user.click(screen.getByText('6 Hours'));
    expect(defaultProps.onPeriodChange).toHaveBeenCalledWith('6h');
  });

  test('shows reset button only when exploring', () => {
    const { rerender } = render(<MessageRateControls {...defaultProps} />);
    
    expect(screen.queryByText(/Reset to default/)).not.toBeInTheDocument();

    rerender(<MessageRateControls {...defaultProps} isExploring={true} />);
    expect(screen.getByText(/Reset to default/)).toBeInTheDocument();
  });
});
