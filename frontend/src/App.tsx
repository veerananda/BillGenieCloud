import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import MenuManagement from './pages/MenuManagement';
import OrdersPage from './pages/OrdersPage';
import './App.css';

const PrivateRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const token = localStorage.getItem('token');
  return token ? <>{children}</> : <Navigate to="/login" />;
};

const Layout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const user = JSON.parse(localStorage.getItem('user') || '{}');
  
  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    window.location.href = '/login';
  };

  return (
    <div className="app-layout">
      <nav className="sidebar">
        <div className="sidebar-header">
          <h2>BillGenie</h2>
          <p>{user.firstName} {user.lastName}</p>
          <span className="user-role">{user.role}</span>
        </div>
        <ul className="nav-menu">
          <li><a href="/dashboard">Dashboard</a></li>
          <li><a href="/menu">Menu</a></li>
          <li><a href="/orders">Orders</a></li>
          <li><a href="#" onClick={handleLogout}>Logout</a></li>
        </ul>
      </nav>
      <main className="main-content">
        {children}
      </main>
    </div>
  );
};

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/dashboard" element={
          <PrivateRoute>
            <Layout><Dashboard /></Layout>
          </PrivateRoute>
        } />
        <Route path="/menu" element={
          <PrivateRoute>
            <Layout><MenuManagement /></Layout>
          </PrivateRoute>
        } />
        <Route path="/orders" element={
          <PrivateRoute>
            <Layout><OrdersPage /></Layout>
          </PrivateRoute>
        } />
        <Route path="/" element={<Navigate to="/dashboard" />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
