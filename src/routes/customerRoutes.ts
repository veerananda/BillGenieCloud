import express from 'express';
import {
  createCustomer,
  getAllCustomers,
  getCustomerById,
  getCustomerOrders,
  updateCustomer,
  deleteCustomer
} from '../controllers/customerController';
import { authenticate, authorize } from '../middleware/auth';

const router = express.Router();

router.post('/', authenticate, createCustomer);
router.get('/', authenticate, getAllCustomers);
router.get('/:id', authenticate, getCustomerById);
router.get('/:id/orders', authenticate, getCustomerOrders);
router.put('/:id', authenticate, updateCustomer);
router.delete('/:id', authenticate, authorize('admin', 'manager'), deleteCustomer);

export default router;
