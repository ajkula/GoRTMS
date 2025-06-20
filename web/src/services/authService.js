import { forceLogout } from '../hooks/useAuth';
const API_BASE = '/api';

class AuthService {
  constructor() {
    this.tokenKey = 'gortms_auth_token';
    this.currentUserCache = null;
    this.lastTokenCheck = null;
  }

  setToken(token) {
    if (token) {
      localStorage.setItem(this.tokenKey, token);
    } else {
      localStorage.removeItem(this.tokenKey);

    }
    this.currentUserCache = null;
    this.lastTokenCheck = null;
  }

  getToken() {
    return localStorage.getItem(this.tokenKey);
  }

  removeToken() {
    localStorage.removeItem(this.tokenKey);
    this.currentUserCache = null;
    this.lastTokenCheck = null;
  }

  isTokenExpired(token) {
    if (!token) return true;

    try {
      const payload = JSON.parse(atob(token.split('.')[1]));
      const now = Date.now() / 1000;
      return payload.exp < now;
    } catch {
      return true;
    }
  }

  async login(username, password) {
    const response = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ username, password }),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      this.removeToken();
      throw new Error(error.message || 'Login failed');
    }

    const data = await response.json();
    this.setToken(data.token);

    return {
      token: data.token,
      user: data.user
    };
  }

  handleAuthenticationError() {
    console.warn('Token expired or invalid, cleaning auth state');
    this.removeToken();
    forceLogout();
  }

  logout() {
    this.removeToken();
  }

  async getCurrentUser() {
    const token = this.getToken();

    if (!token || this.isTokenExpired(token)) {
      this.removeToken();
      return null;
    }

    if (this.currentUserCache && this.lastTokenCheck === token) {
      return this.currentUserCache;
    }

    try {
      const response = await fetch(`${API_BASE}/auth/profile`, {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        this.removeToken();
        return null;
      }

      const user = await response.json();
      this.currentUserCache = user;
      this.lastTokenCheck = token;
      return user;
    } catch {
      this.removeToken();
      return null;
    }
  }

  async updateUser(userId, updates) {
    const token = this.getToken();
    const response = await fetch(`${API_BASE}/users/${userId}`, {
      method: 'PATCH',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(updates),
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || 'Failed to update user');
    }

    const data = await response.json();

    if (data.token) {
      this.setToken(data.token);
    }

    return data;
  }

  hasRole(user, requiredRole) {
    if (!user || !user.role) return false;

    if (user.role === 'admin') return true;

    return user.role === requiredRole;
  }

  getAuthHeaders() {
    const token = this.getToken();
    return token ? { 'Authorization': `Bearer ${token}` } : {};
  }
}

export const authService = new AuthService();
