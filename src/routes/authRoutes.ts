import express from 'express';
import {
  register,
  login,
  getAllUsers,
  getUserById,
  updateUser,
  deleteUser
} from '../controllers/authController';
import { authenticate, authorize } from '../middleware/auth';

const router = express.Router();

router.post('/register', authenticate, authorize('admin', 'manager'), register);
router.post('/login', login);
router.get('/users', authenticate, authorize('admin', 'manager'), getAllUsers);
router.get('/users/:id', authenticate, getUserById);
router.put('/users/:id', authenticate, authorize('admin', 'manager'), updateUser);
router.delete('/users/:id', authenticate, authorize('admin'), deleteUser);

export default router;
