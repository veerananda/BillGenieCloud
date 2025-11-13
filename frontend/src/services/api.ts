import axios from 'axios';

const API_BASE_URL = '/api';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
});

// Add token to requests if available
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Auth API
export const authAPI = {
  login: (username: string, password: string) => 
    api.post('/auth/login', { username, password }),
  register: (userData: any) => 
    api.post('/auth/register', userData)
};

// Menu API
export const menuAPI = {
  getAll: (params?: any) => api.get('/menu', { params }),
  getById: (id: string) => api.get(`/menu/${id}`),
  create: (data: any) => api.post('/menu', data),
  update: (id: string, data: any) => api.put(`/menu/${id}`, data),
  delete: (id: string) => api.delete(`/menu/${id}`)
};

// Orders API
export const ordersAPI = {
  getAll: (params?: any) => api.get('/orders', { params }),
  getById: (id: string) => api.get(`/orders/${id}`),
  create: (data: any) => api.post('/orders', data),
  updateStatus: (id: string, status: string) => 
    api.patch(`/orders/${id}/status`, { status }),
  updatePayment: (id: string, paymentStatus: string, paymentMethod?: string) => 
    api.patch(`/orders/${id}/payment`, { paymentStatus, paymentMethod }),
  cancel: (id: string) => api.patch(`/orders/${id}/cancel`)
};

// Customers API
export const customersAPI = {
  getAll: () => api.get('/customers'),
  getById: (id: string) => api.get(`/customers/${id}`),
  getOrders: (id: string) => api.get(`/customers/${id}/orders`),
  create: (data: any) => api.post('/customers', data),
  update: (id: string, data: any) => api.put(`/customers/${id}`, data),
  delete: (id: string) => api.delete(`/customers/${id}`)
};

// Inventory API
export const inventoryAPI = {
  getAll: (params?: any) => api.get('/inventory', { params }),
  getById: (id: string) => api.get(`/inventory/${id}`),
  create: (data: any) => api.post('/inventory', data),
  update: (id: string, data: any) => api.put(`/inventory/${id}`, data),
  restock: (id: string, quantity: number) => 
    api.patch(`/inventory/${id}/restock`, { quantity }),
  delete: (id: string) => api.delete(`/inventory/${id}`)
};

// Tables API
export const tablesAPI = {
  getAll: (params?: any) => api.get('/tables', { params }),
  getById: (id: string) => api.get(`/tables/${id}`),
  create: (data: any) => api.post('/tables', data),
  updateStatus: (id: string, status: string, currentOrderId?: string) => 
    api.patch(`/tables/${id}/status`, { status, currentOrderId }),
  update: (id: string, data: any) => api.put(`/tables/${id}`, data),
  delete: (id: string) => api.delete(`/tables/${id}`)
};

// Reservations API
export const reservationsAPI = {
  getAll: (params?: any) => api.get('/reservations', { params }),
  getById: (id: string) => api.get(`/reservations/${id}`),
  create: (data: any) => api.post('/reservations', data),
  updateStatus: (id: string, status: string) => 
    api.patch(`/reservations/${id}/status`, { status }),
  delete: (id: string) => api.delete(`/reservations/${id}`)
};

// Analytics API
export const analyticsAPI = {
  getSalesReport: (params?: any) => api.get('/analytics/sales', { params }),
  getPopularItems: (limit?: number) => 
    api.get('/analytics/popular-items', { params: { limit } }),
  getCustomerAnalytics: () => api.get('/analytics/customers'),
  getDashboard: () => api.get('/analytics/dashboard')
};

export default api;
