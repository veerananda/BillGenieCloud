import { Request, Response } from 'express';
import Reservation from '../models/Reservation';
import Table from '../models/Table';

export const createReservation = async (req: Request, res: Response): Promise<void> => {
  try {
    // Check if table is available for the requested time
    const existingReservation = await Reservation.findOne({
      tableId: req.body.tableId,
      reservationDate: req.body.reservationDate,
      status: { $in: ['pending', 'confirmed', 'seated'] }
    });
    
    if (existingReservation) {
      res.status(400).json({ 
        success: false, 
        error: 'Table is already reserved for this time slot' 
      });
      return;
    }
    
    const reservation = new Reservation(req.body);
    await reservation.save();
    
    // Update table status
    await Table.findByIdAndUpdate(req.body.tableId, { status: 'reserved' });
    
    await reservation.populate(['customerId', 'tableId']);
    res.status(201).json({ success: true, data: reservation });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getAllReservations = async (req: Request, res: Response): Promise<void> => {
  try {
    const { status, date } = req.query;
    const filter: any = {};
    
    if (status) filter.status = status;
    if (date) {
      const startDate = new Date(date as string);
      const endDate = new Date(startDate);
      endDate.setDate(endDate.getDate() + 1);
      filter.reservationDate = { $gte: startDate, $lt: endDate };
    }
    
    const reservations = await Reservation.find(filter)
      .populate('customerId')
      .populate('tableId')
      .sort({ reservationDate: 1 });
    
    res.status(200).json({ success: true, data: reservations });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const getReservationById = async (req: Request, res: Response): Promise<void> => {
  try {
    const reservation = await Reservation.findById(req.params.id)
      .populate('customerId')
      .populate('tableId');
    
    if (!reservation) {
      res.status(404).json({ success: false, error: 'Reservation not found' });
      return;
    }
    res.status(200).json({ success: true, data: reservation });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const updateReservationStatus = async (req: Request, res: Response): Promise<void> => {
  try {
    const { status } = req.body;
    const reservation = await Reservation.findByIdAndUpdate(
      req.params.id,
      { status },
      { new: true, runValidators: true }
    ).populate(['customerId', 'tableId']);
    
    if (!reservation) {
      res.status(404).json({ success: false, error: 'Reservation not found' });
      return;
    }
    
    // Update table status based on reservation status
    if (status === 'seated') {
      await Table.findByIdAndUpdate(reservation.tableId, { status: 'occupied' });
    } else if (status === 'completed' || status === 'cancelled') {
      await Table.findByIdAndUpdate(reservation.tableId, { status: 'available' });
    }
    
    res.status(200).json({ success: true, data: reservation });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};

export const deleteReservation = async (req: Request, res: Response): Promise<void> => {
  try {
    const reservation = await Reservation.findByIdAndDelete(req.params.id);
    if (!reservation) {
      res.status(404).json({ success: false, error: 'Reservation not found' });
      return;
    }
    
    // Update table status
    await Table.findByIdAndUpdate(reservation.tableId, { status: 'available' });
    
    res.status(200).json({ success: true, message: 'Reservation deleted successfully' });
  } catch (error: any) {
    res.status(400).json({ success: false, error: error.message });
  }
};
