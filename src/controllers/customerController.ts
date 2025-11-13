import { Request, Response } from 'express';
import Customer from '../models/Customer';
import Order from '../models/Order';

export const createCustomer = async (req: Request, res: Response): Promise<void> => {
  try {
    const customer = new Customer(req.body);
    await customer.save();
    res.status(201).json({ success: true, data: customer });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getAllCustomers = async (req: Request, res: Response): Promise<void> => {
  try {
    const customers = await Customer.find().sort({ createdAt: -1 });
    res.status(200).json({ success: true, data: customers });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getCustomerById = async (req: Request, res: Response): Promise<void> => {
  try {
    const customer = await Customer.findById(req.params.id);
    if (!customer) {
      res.status(404).json({ success: false, error: 'Customer not found' });
      return;
    }
    res.status(200).json({ success: true, data: customer });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getCustomerOrders = async (req: Request, res: Response): Promise<void> => {
  try {
    const orders = await Order.find({ customerId: req.params.id })
      .populate('items.menuItem')
      .sort({ createdAt: -1 });
    
    res.status(200).json({ success: true, data: orders });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updateCustomer = async (req: Request, res: Response): Promise<void> => {
  try {
    const customer = await Customer.findByIdAndUpdate(
      req.params.id,
      req.body,
      { new: true, runValidators: true }
    );
    if (!customer) {
      res.status(404).json({ success: false, error: 'Customer not found' });
      return;
    }
    res.status(200).json({ success: true, data: customer });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const deleteCustomer = async (req: Request, res: Response): Promise<void> => {
  try {
    const customer = await Customer.findByIdAndDelete(req.params.id);
    if (!customer) {
      res.status(404).json({ success: false, error: 'Customer not found' });
      return;
    }
    res.status(200).json({ success: true, message: 'Customer deleted successfully' });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};
