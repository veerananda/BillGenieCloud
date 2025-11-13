import express from 'express';
import {
  createReservation,
  getAllReservations,
  getReservationById,
  updateReservationStatus,
  deleteReservation
} from '../controllers/reservationController';
import { authenticate, authorize } from '../middleware/auth';

const router = express.Router();

router.post('/', authenticate, createReservation);
router.get('/', authenticate, getAllReservations);
router.get('/:id', authenticate, getReservationById);
router.patch('/:id/status', authenticate, updateReservationStatus);
router.delete('/:id', authenticate, authorize('admin', 'manager'), deleteReservation);

export default router;
