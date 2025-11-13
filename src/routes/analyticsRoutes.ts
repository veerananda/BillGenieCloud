import express from 'express';
import {
  getSalesReport,
  getPopularItems,
  getCustomerAnalytics,
  getDashboardStats
} from '../controllers/analyticsController';
import { authenticate, authorize } from '../middleware/auth';

const router = express.Router();

router.get('/sales', authenticate, authorize('admin', 'manager'), getSalesReport);
router.get('/popular-items', authenticate, authorize('admin', 'manager'), getPopularItems);
router.get('/customers', authenticate, authorize('admin', 'manager'), getCustomerAnalytics);
router.get('/dashboard', authenticate, getDashboardStats);

export default router;
