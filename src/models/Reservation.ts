import mongoose, { Document, Schema } from 'mongoose';

export interface IReservation extends Document {
  customerId: mongoose.Types.ObjectId;
  tableId: mongoose.Types.ObjectId;
  reservationDate: Date;
  numberOfGuests: number;
  status: 'pending' | 'confirmed' | 'seated' | 'completed' | 'cancelled';
  specialRequests?: string;
  createdAt: Date;
  updatedAt: Date;
}

const ReservationSchema = new Schema<IReservation>({
  customerId: { type: Schema.Types.ObjectId, ref: 'Customer', required: true },
  tableId: { type: Schema.Types.ObjectId, ref: 'Table', required: true },
  reservationDate: { type: Date, required: true },
  numberOfGuests: { type: Number, required: true, min: 1 },
  status: {
    type: String,
    enum: ['pending', 'confirmed', 'seated', 'completed', 'cancelled'],
    default: 'pending'
  },
  specialRequests: { type: String }
}, {
  timestamps: true
});

export default mongoose.model<IReservation>('Reservation', ReservationSchema);
