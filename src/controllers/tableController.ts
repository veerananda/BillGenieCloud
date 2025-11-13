import { Request, Response } from 'express';
import Table from '../models/Table';

export const createTable = async (req: Request, res: Response): Promise<void> => {
  try {
    const table = new Table(req.body);
    await table.save();
    res.status(201).json({ success: true, data: table });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getAllTables = async (req: Request, res: Response): Promise<void> => {
  try {
    const { status, location } = req.query;
    const filter: any = {};
    
    if (status) filter.status = status;
    if (location) filter.location = location;
    
    const tables = await Table.find(filter)
      .populate('currentOrderId')
      .sort({ tableNumber: 1 });
    
    res.status(200).json({ success: true, data: tables });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getTableById = async (req: Request, res: Response): Promise<void> => {
  try {
    const table = await Table.findById(req.params.id).populate('currentOrderId');
    if (!table) {
      res.status(404).json({ success: false, error: 'Table not found' });
      return;
    }
    res.status(200).json({ success: true, data: table });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updateTableStatus = async (req: Request, res: Response): Promise<void> => {
  try {
    const { status, currentOrderId } = req.body;
    const updateData: any = { status };
    
    if (currentOrderId !== undefined) {
      updateData.currentOrderId = currentOrderId || null;
    }
    
    const table = await Table.findByIdAndUpdate(
      req.params.id,
      updateData,
      { new: true, runValidators: true }
    );
    
    if (!table) {
      res.status(404).json({ success: false, error: 'Table not found' });
      return;
    }
    res.status(200).json({ success: true, data: table });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updateTable = async (req: Request, res: Response): Promise<void> => {
  try {
    const table = await Table.findByIdAndUpdate(
      req.params.id,
      req.body,
      { new: true, runValidators: true }
    );
    if (!table) {
      res.status(404).json({ success: false, error: 'Table not found' });
      return;
    }
    res.status(200).json({ success: true, data: table });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const deleteTable = async (req: Request, res: Response): Promise<void> => {
  try {
    const table = await Table.findByIdAndDelete(req.params.id);
    if (!table) {
      res.status(404).json({ success: false, error: 'Table not found' });
      return;
    }
    res.status(200).json({ success: true, message: 'Table deleted successfully' });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};
