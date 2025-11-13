import { Request, Response } from 'express';
import Order from '../models/Order';
import Customer from '../models/Customer';
import MenuItem from '../models/MenuItem';

export const getSalesReport = async (req: Request, res: Response): Promise<void> => {
  try {
    const { startDate, endDate } = req.query;
    const filter: any = { paymentStatus: 'paid' };
    
    if (startDate || endDate) {
      filter.createdAt = {};
      if (startDate) filter.createdAt.$gte = new Date(startDate as string);
      if (endDate) filter.createdAt.$lte = new Date(endDate as string);
    }
    
    const orders = await Order.find(filter);
    
    const totalRevenue = orders.reduce((sum, order) => sum + order.total, 0);
    const totalOrders = orders.length;
    const averageOrderValue = totalOrders > 0 ? totalRevenue / totalOrders : 0;
    
    const ordersByType = orders.reduce((acc: any, order) => {
      acc[order.orderType] = (acc[order.orderType] || 0) + 1;
      return acc;
    }, {});
    
    res.status(200).json({
      success: true,
      data: {
        totalRevenue,
        totalOrders,
        averageOrderValue,
        ordersByType
      }
    });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getPopularItems = async (req: Request, res: Response): Promise<void> => {
  try {
    const { limit = '10' } = req.query;
    
    const popularItems = await Order.aggregate([
      { $match: { status: { $ne: 'cancelled' } } },
      { $unwind: '$items' },
      {
        $group: {
          _id: '$items.menuItem',
          totalOrders: { $sum: 1 },
          totalQuantity: { $sum: '$items.quantity' },
          totalRevenue: { $sum: { $multiply: ['$items.price', '$items.quantity'] } }
        }
      },
      { $sort: { totalQuantity: -1 } },
      { $limit: parseInt(limit as string) }
    ]);
    
    // Populate menu item details
    const populatedItems = await MenuItem.populate(popularItems, { path: '_id' });
    
    res.status(200).json({ success: true, data: populatedItems });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getCustomerAnalytics = async (req: Request, res: Response): Promise<void> => {
  try {
    const totalCustomers = await Customer.countDocuments();
    const topCustomers = await Customer.find()
      .sort({ totalSpent: -1 })
      .limit(10)
      .select('firstName lastName email totalOrders totalSpent loyaltyPoints');
    
    const avgOrdersPerCustomer = await Customer.aggregate([
      { $group: { _id: null, avgOrders: { $avg: '$totalOrders' } } }
    ]);
    
    res.status(200).json({
      success: true,
      data: {
        totalCustomers,
        topCustomers,
        averageOrdersPerCustomer: avgOrdersPerCustomer[0]?.avgOrders || 0
      }
    });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getDashboardStats = async (req: Request, res: Response): Promise<void> => {
  try {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    
    const todayOrders = await Order.countDocuments({
      createdAt: { $gte: today }
    });
    
    const todayRevenue = await Order.aggregate([
      { $match: { createdAt: { $gte: today }, paymentStatus: 'paid' } },
      { $group: { _id: null, total: { $sum: '$total' } } }
    ]);
    
    const activeOrders = await Order.countDocuments({
      status: { $in: ['pending', 'preparing', 'ready'] }
    });
    
    const totalCustomers = await Customer.countDocuments();
    
    res.status(200).json({
      success: true,
      data: {
        todayOrders,
        todayRevenue: todayRevenue[0]?.total || 0,
        activeOrders,
        totalCustomers
      }
    });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};
