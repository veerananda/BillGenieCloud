import React, { useState, useEffect } from 'react';
import { analyticsAPI } from '../services/api';

const Dashboard: React.FC = () => {
  const [stats, setStats] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadDashboard();
  }, []);

  const loadDashboard = async () => {
    try {
      const response = await analyticsAPI.getDashboard();
      setStats(response.data.data);
    } catch (error) {
      console.error('Error loading dashboard:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return <div className="loading">Loading dashboard...</div>;
  }

  return (
    <div className="dashboard">
      <h1>Restaurant Dashboard</h1>
      
      <div className="stats-grid">
        <div className="stat-card">
          <h3>Today's Orders</h3>
          <p className="stat-value">{stats?.todayOrders || 0}</p>
        </div>
        
        <div className="stat-card">
          <h3>Today's Revenue</h3>
          <p className="stat-value">${stats?.todayRevenue?.toFixed(2) || '0.00'}</p>
        </div>
        
        <div className="stat-card">
          <h3>Active Orders</h3>
          <p className="stat-value">{stats?.activeOrders || 0}</p>
        </div>
        
        <div className="stat-card">
          <h3>Total Customers</h3>
          <p className="stat-value">{stats?.totalCustomers || 0}</p>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
