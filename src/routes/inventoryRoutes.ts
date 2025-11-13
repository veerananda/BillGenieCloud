import express from 'express';
import {
  createInventoryItem,
  getAllInventoryItems,
  getInventoryItemById,
  updateInventoryItem,
  restockInventory,
  deleteInventoryItem
} from '../controllers/inventoryController';
import { authenticate, authorize } from '../middleware/auth';

const router = express.Router();

router.post('/', authenticate, authorize('admin', 'manager'), createInventoryItem);
router.get('/', authenticate, getAllInventoryItems);
router.get('/:id', authenticate, getInventoryItemById);
router.put('/:id', authenticate, authorize('admin', 'manager'), updateInventoryItem);
router.patch('/:id/restock', authenticate, authorize('admin', 'manager'), restockInventory);
router.delete('/:id', authenticate, authorize('admin', 'manager'), deleteInventoryItem);

export default router;
