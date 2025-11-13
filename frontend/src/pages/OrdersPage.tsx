import React, { useState, useEffect } from 'react';
import { ordersAPI } from '../services/api';

const OrdersPage: React.FC = () => {
  const [orders, setOrders] = useState<any[]>([]);
  const [filter, setFilter] = useState('all');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadOrders();
  }, [filter]);

  const loadOrders = async () => {
    try {
      const params = filter !== 'all' ? { status: filter } : {};
      const response = await ordersAPI.getAll(params);
      setOrders(response.data.data);
    } catch (error) {
      console.error('Error loading orders:', error);
    } finally {
      setLoading(false);
    }
  };

  const updateStatus = async (orderId: string, newStatus: string) => {
    try {
      await ordersAPI.updateStatus(orderId, newStatus);
      loadOrders();
    } catch (error) {
      console.error('Error updating order:', error);
    }
  };

  const getStatusColor = (status: string) => {
    const colors: Record<string, string> = {
      pending: 'orange',
      preparing: 'blue',
      ready: 'purple',
      served: 'green',
      completed: 'gray',
      cancelled: 'red'
    };
    return colors[status] || 'gray';
  };

  if (loading) {
    return <div className="loading">Loading orders...</div>;
  }

  return (
    <div className="orders-page">
      <h1>Orders Management</h1>

      <div className="filters">
        <button 
          className={filter === 'all' ? 'active' : ''}
          onClick={() => setFilter('all')}
        >
          All Orders
        </button>
        <button 
          className={filter === 'pending' ? 'active' : ''}
          onClick={() => setFilter('pending')}
        >
          Pending
        </button>
        <button 
          className={filter === 'preparing' ? 'active' : ''}
          onClick={() => setFilter('preparing')}
        >
          Preparing
        </button>
        <button 
          className={filter === 'ready' ? 'active' : ''}
          onClick={() => setFilter('ready')}
        >
          Ready
        </button>
      </div>

      <div className="orders-grid">
        {orders.map((order) => (
          <div key={order._id} className="order-card">
            <div className="order-header">
              <h3>Order #{order.orderNumber}</h3>
              <span 
                className="status-badge" 
                style={{ backgroundColor: getStatusColor(order.status) }}
              >
                {order.status}
              </span>
            </div>
            
            <div className="order-details">
              <p><strong>Type:</strong> {order.orderType}</p>
              {order.tableNumber && <p><strong>Table:</strong> {order.tableNumber}</p>}
              <p><strong>Items:</strong> {order.items.length}</p>
              <p><strong>Total:</strong> ${order.total.toFixed(2)}</p>
              <p><strong>Payment:</strong> {order.paymentStatus}</p>
            </div>

            <div className="order-actions">
              {order.status === 'pending' && (
                <button onClick={() => updateStatus(order._id, 'preparing')}>
                  Start Preparing
                </button>
              )}
              {order.status === 'preparing' && (
                <button onClick={() => updateStatus(order._id, 'ready')}>
                  Mark Ready
                </button>
              )}
              {order.status === 'ready' && (
                <button onClick={() => updateStatus(order._id, 'served')}>
                  Mark Served
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {orders.length === 0 && (
        <div className="no-data">No orders found</div>
      )}
    </div>
  );
};

export default OrdersPage;
