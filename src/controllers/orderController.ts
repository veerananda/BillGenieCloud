import { Request, Response } from 'express';
import Order from '../models/Order';
import Customer from '../models/Customer';

// Generate unique order number
const generateOrderNumber = (): string => {
  const timestamp = Date.now().toString().slice(-6);
  const random = Math.floor(Math.random() * 1000).toString().padStart(3, '0');
  return `ORD${timestamp}${random}`;
};

export const createOrder = async (req: Request, res: Response): Promise<void> => {
  try {
    const orderData = {
      ...req.body,
      orderNumber: generateOrderNumber()
    };
    
    const order = new Order(orderData);
    await order.save();
    
    // Update customer stats if customerId provided
    if (order.customerId) {
      await Customer.findByIdAndUpdate(order.customerId, {
        $inc: { totalOrders: 1, totalSpent: order.total, loyaltyPoints: Math.floor(order.total) }
      });
    }
    
    await order.populate('items.menuItem');
    res.status(201).json({ success: true, data: order });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getAllOrders = async (req: Request, res: Response): Promise<void> => {
  try {
    const { status, orderType, tableNumber } = req.query;
    const filter: any = {};
    
    if (status) filter.status = status;
    if (orderType) filter.orderType = orderType;
    if (tableNumber) filter.tableNumber = parseInt(tableNumber as string);
    
    const orders = await Order.find(filter)
      .populate('items.menuItem')
      .populate('customerId')
      .sort({ createdAt: -1 });
    
    res.status(200).json({ success: true, data: orders });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getOrderById = async (req: Request, res: Response): Promise<void> => {
  try {
    const order = await Order.findById(req.params.id)
      .populate('items.menuItem')
      .populate('customerId');
    
    if (!order) {
      res.status(404).json({ success: false, error: 'Order not found' });
      return;
    }
    res.status(200).json({ success: true, data: order });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updateOrderStatus = async (req: Request, res: Response): Promise<void> => {
  try {
    const { status } = req.body;
    const order = await Order.findByIdAndUpdate(
      req.params.id,
      { status },
      { new: true, runValidators: true }
    ).populate('items.menuItem');
    
    if (!order) {
      res.status(404).json({ success: false, error: 'Order not found' });
      return;
    }
    res.status(200).json({ success: true, data: order });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updatePaymentStatus = async (req: Request, res: Response): Promise<void> => {
  try {
    const { paymentStatus, paymentMethod } = req.body;
    const order = await Order.findByIdAndUpdate(
      req.params.id,
      { paymentStatus, paymentMethod },
      { new: true, runValidators: true }
    );
    
    if (!order) {
      res.status(404).json({ success: false, error: 'Order not found' });
      return;
    }
    res.status(200).json({ success: true, data: order });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const cancelOrder = async (req: Request, res: Response): Promise<void> => {
  try {
    const order = await Order.findById(req.params.id);
    if (!order) {
      res.status(404).json({ success: false, error: 'Order not found' });
      return;
    }
    
    if (order.paymentStatus === 'paid') {
      res.status(400).json({ success: false, error: 'Cannot cancel paid order. Refund required.' });
      return;
    }
    
    order.status = 'cancelled';
    await order.save();
    
    res.status(200).json({ success: true, data: order });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};
