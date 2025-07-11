export default {
  getDomains: jest.fn(),
  createDomain: jest.fn(),
  deleteDomain: jest.fn(),
  getQueues: jest.fn(),
  createQueue: jest.fn(),
  deleteQueue: jest.fn(),
  publishMessage: jest.fn(),
  consumeMessages: jest.fn(),
  getStats: jest.fn(),
  getStatsHistory: jest.fn(),
  getCurrentStats: jest.fn(),
  formatHistoryForCharts: jest.fn(),
  getNotifications: jest.fn(),
  markNotificationAsRead: jest.fn(),
  // service account
  getServices: jest.fn(),
  createService: jest.fn(),
  getService: jest.fn(),
  deleteService: jest.fn(),
  rotateServiceSecret: jest.fn(),
  updateServicePermissions: jest.fn(),
};
