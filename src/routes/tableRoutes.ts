import express from 'express';
import {
  createTable,
  getAllTables,
  getTableById,
  updateTableStatus,
  updateTable,
  deleteTable
} from '../controllers/tableController';
import { authenticate, authorize } from '../middleware/auth';

const router = express.Router();

router.post('/', authenticate, authorize('admin', 'manager'), createTable);
router.get('/', authenticate, getAllTables);
router.get('/:id', authenticate, getTableById);
router.patch('/:id/status', authenticate, updateTableStatus);
router.put('/:id', authenticate, authorize('admin', 'manager'), updateTable);
router.delete('/:id', authenticate, authorize('admin', 'manager'), deleteTable);

export default router;
