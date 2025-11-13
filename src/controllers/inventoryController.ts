import { Request, Response } from 'express';
import Inventory from '../models/Inventory';

export const createInventoryItem = async (req: Request, res: Response): Promise<void> => {
  try {
    const item = new Inventory(req.body);
    await item.save();
    res.status(201).json({ success: true, data: item });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getAllInventoryItems = async (req: Request, res: Response): Promise<void> => {
  try {
    const { category, lowStock } = req.query;
    const filter: any = {};
    
    if (category) filter.category = category;
    
    let items = await Inventory.find(filter).sort({ itemName: 1 });
    
    if (lowStock === 'true') {
      items = items.filter(item => item.quantity <= item.reorderLevel);
    }
    
    res.status(200).json({ success: true, data: items });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getInventoryItemById = async (req: Request, res: Response): Promise<void> => {
  try {
    const item = await Inventory.findById(req.params.id);
    if (!item) {
      res.status(404).json({ success: false, error: 'Inventory item not found' });
      return;
    }
    res.status(200).json({ success: true, data: item });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updateInventoryItem = async (req: Request, res: Response): Promise<void> => {
  try {
    const item = await Inventory.findByIdAndUpdate(
      req.params.id,
      req.body,
      { new: true, runValidators: true }
    );
    if (!item) {
      res.status(404).json({ success: false, error: 'Inventory item not found' });
      return;
    }
    res.status(200).json({ success: true, data: item });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const restockInventory = async (req: Request, res: Response): Promise<void> => {
  try {
    const { quantity } = req.body;
    const item = await Inventory.findById(req.params.id);
    
    if (!item) {
      res.status(404).json({ success: false, error: 'Inventory item not found' });
      return;
    }
    
    item.quantity += quantity;
    item.lastRestocked = new Date();
    await item.save();
    
    res.status(200).json({ success: true, data: item });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const deleteInventoryItem = async (req: Request, res: Response): Promise<void> => {
  try {
    const item = await Inventory.findByIdAndDelete(req.params.id);
    if (!item) {
      res.status(404).json({ success: false, error: 'Inventory item not found' });
      return;
    }
    res.status(200).json({ success: true, message: 'Inventory item deleted successfully' });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};
