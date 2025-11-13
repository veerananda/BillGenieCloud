import mongoose, { Document, Schema } from 'mongoose';

export interface ITable extends Document {
  tableNumber: number;
  capacity: number;
  status: 'available' | 'occupied' | 'reserved' | 'cleaning';
  currentOrderId?: mongoose.Types.ObjectId;
  location: string;
  createdAt: Date;
  updatedAt: Date;
}

const TableSchema = new Schema<ITable>({
  tableNumber: { type: Number, required: true, unique: true },
  capacity: { type: Number, required: true, min: 1 },
  status: {
    type: String,
    enum: ['available', 'occupied', 'reserved', 'cleaning'],
    default: 'available'
  },
  currentOrderId: { type: Schema.Types.ObjectId, ref: 'Order' },
  location: { type: String, required: true }
}, {
  timestamps: true
});

export default mongoose.model<ITable>('Table', TableSchema);
