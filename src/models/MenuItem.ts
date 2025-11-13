import mongoose, { Document, Schema } from 'mongoose';

export interface IMenuItem extends Document {
  name: string;
  description: string;
  category: string;
  price: number;
  imageUrl?: string;
  available: boolean;
  preparationTime: number; // in minutes
  ingredients: string[];
  allergens?: string[];
  nutritionalInfo?: {
    calories: number;
    protein: number;
    carbs: number;
    fat: number;
  };
  createdAt: Date;
  updatedAt: Date;
}

const MenuItemSchema = new Schema<IMenuItem>({
  name: { type: String, required: true, trim: true },
  description: { type: String, required: true },
  category: { type: String, required: true },
  price: { type: Number, required: true, min: 0 },
  imageUrl: { type: String },
  available: { type: Boolean, default: true },
  preparationTime: { type: Number, required: true, min: 0 },
  ingredients: [{ type: String }],
  allergens: [{ type: String }],
  nutritionalInfo: {
    calories: { type: Number },
    protein: { type: Number },
    carbs: { type: Number },
    fat: { type: Number }
  }
}, {
  timestamps: true
});

export default mongoose.model<IMenuItem>('MenuItem', MenuItemSchema);
