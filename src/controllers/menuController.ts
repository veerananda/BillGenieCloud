import { Request, Response } from 'express';
import MenuItem from '../models/MenuItem';

export const createMenuItem = async (req: Request, res: Response): Promise<void> => {
  try {
    const menuItem = new MenuItem(req.body);
    await menuItem.save();
    res.status(201).json({ success: true, data: menuItem });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getAllMenuItems = async (req: Request, res: Response): Promise<void> => {
  try {
    const { category, available } = req.query;
    const filter: any = {};
    
    if (category) filter.category = category;
    if (available !== undefined) filter.available = available === 'true';
    
    const menuItems = await MenuItem.find(filter).sort({ category: 1, name: 1 });
    res.status(200).json({ success: true, data: menuItems });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getMenuItemById = async (req: Request, res: Response): Promise<void> => {
  try {
    const menuItem = await MenuItem.findById(req.params.id);
    if (!menuItem) {
      res.status(404).json({ success: false, error: 'Menu item not found' });
      return;
    }
    res.status(200).json({ success: true, data: menuItem });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updateMenuItem = async (req: Request, res: Response): Promise<void> => {
  try {
    const menuItem = await MenuItem.findByIdAndUpdate(
      req.params.id,
      req.body,
      { new: true, runValidators: true }
    );
    if (!menuItem) {
      res.status(404).json({ success: false, error: 'Menu item not found' });
      return;
    }
    res.status(200).json({ success: true, data: menuItem });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const deleteMenuItem = async (req: Request, res: Response): Promise<void> => {
  try {
    const menuItem = await MenuItem.findByIdAndDelete(req.params.id);
    if (!menuItem) {
      res.status(404).json({ success: false, error: 'Menu item not found' });
      return;
    }
    res.status(200).json({ success: true, message: 'Menu item deleted successfully' });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};
