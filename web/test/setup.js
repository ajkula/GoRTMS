import '@testing-library/jest-dom';

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: jest.fn().mockImplementation(query => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: jest.fn(),
    removeListener: jest.fn(),
    addEventListener: jest.fn(),
    removeEventListener: jest.fn(),
    dispatchEvent: jest.fn(),
  })),
});

// Suppress React act() warnings in tests
const originalError = console.error;
console.error = (...args) => {
  // Si console.error est mocké (par jest.spyOn), utiliser le mock
  if (console.error !== originalError && typeof console.error.mockImplementation === 'function') {
    return;
  }
  
  const message = args[0];
  if (
    typeof message === 'string' &&
    (message.includes('Warning: An update to TestComponent inside a test was not wrapped in act') ||
     message.includes('Warning: The current testing environment is not configured to support act') ||
     message.includes('When testing, code that causes React state updates should be wrapped into act'))
  ) {
    return;
  }
  originalError.call(console, ...args);
};
