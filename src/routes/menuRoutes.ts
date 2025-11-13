import express from 'express';
import {
  createMenuItem,
  getAllMenuItems,
  getMenuItemById,
  updateMenuItem,
  deleteMenuItem
} from '../controllers/menuController';
import { authenticate, authorize } from '../middleware/auth';

const router = express.Router();

router.get('/', getAllMenuItems);
router.get('/:id', getMenuItemById);
router.post('/', authenticate, authorize('admin', 'manager'), createMenuItem);
router.put('/:id', authenticate, authorize('admin', 'manager'), updateMenuItem);
router.delete('/:id', authenticate, authorize('admin', 'manager'), deleteMenuItem);

export default router;
