import mongoose, { Document, Schema } from 'mongoose';

export interface IInventory extends Document {
  itemName: string;
  category: string;
  quantity: number;
  unit: string;
  reorderLevel: number;
  supplier: string;
  costPerUnit: number;
  lastRestocked: Date;
  expiryDate?: Date;
  createdAt: Date;
  updatedAt: Date;
}

const InventorySchema = new Schema<IInventory>({
  itemName: { type: String, required: true, trim: true },
  category: { type: String, required: true },
  quantity: { type: Number, required: true, min: 0 },
  unit: { type: String, required: true },
  reorderLevel: { type: Number, required: true },
  supplier: { type: String, required: true },
  costPerUnit: { type: Number, required: true },
  lastRestocked: { type: Date, default: Date.now },
  expiryDate: { type: Date }
}, {
  timestamps: true
});

export default mongoose.model<IInventory>('Inventory', InventorySchema);
