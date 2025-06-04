export const testEnvironment = 'jsdom';
export const roots = ['<rootDir>/test'];
export const setupFilesAfterEnv = ['<rootDir>/test/setup.js'];
export const moduleNameMapper = {
  '\\.(css|less|scss|sass)$': '<rootDir>/test/__mocks__/styleMock.js',
  '\\.(jpg|jpeg|png|gif|svg)$': '<rootDir>/test/__mocks__/fileMock.js'
};
export const transform = {
  '^.+\\.(js|jsx)$': 'babel-jest',
};
export const testMatch = [
  '<rootDir>/test/**/*.test.js'
];
export const collectCoverageFrom = [
  'src/**/*.{js,jsx}',
  '!src/index.js',
  '!src/**/*.config.js'
];
