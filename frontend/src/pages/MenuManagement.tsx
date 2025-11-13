import React, { useState, useEffect } from 'react';
import { menuAPI } from '../services/api';

const MenuManagement: React.FC = () => {
  const [menuItems, setMenuItems] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadMenu();
  }, []);

  const loadMenu = async () => {
    try {
      const response = await menuAPI.getAll();
      setMenuItems(response.data.data);
    } catch (error) {
      console.error('Error loading menu:', error);
    } finally {
      setLoading(false);
    }
  };

  const toggleAvailability = async (id: string, currentStatus: boolean) => {
    try {
      await menuAPI.update(id, { available: !currentStatus });
      loadMenu();
    } catch (error) {
      console.error('Error updating item:', error);
    }
  };

  if (loading) {
    return <div className="loading">Loading menu...</div>;
  }

  return (
    <div className="menu-management">
      <div className="page-header">
        <h1>Menu Management</h1>
        <button className="btn-primary">Add New Item</button>
      </div>

      <div className="menu-grid">
        {menuItems.map((item) => (
          <div key={item._id} className="menu-card">
            <div className="menu-card-header">
              <h3>{item.name}</h3>
              <span className={`badge ${item.available ? 'badge-success' : 'badge-danger'}`}>
                {item.available ? 'Available' : 'Unavailable'}
              </span>
            </div>
            <p className="menu-description">{item.description}</p>
            <div className="menu-details">
              <span className="category">{item.category}</span>
              <span className="price">${item.price.toFixed(2)}</span>
            </div>
            <div className="menu-actions">
              <button 
                className="btn-secondary"
                onClick={() => toggleAvailability(item._id, item.available)}
              >
                Toggle Availability
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default MenuManagement;
