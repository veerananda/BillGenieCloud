require('dotenv').config();
const mongoose = require('mongoose');
const bcrypt = require('bcryptjs');

async function createAdmin() {
  try {
    const mongoURI = process.env.MONGODB_URI || 'mongodb://localhost:27017/billgenie';
    console.log('Connecting to MongoDB...');
    await mongoose.connect(mongoURI);
    console.log('Connected to MongoDB');
    
    // Define User schema inline to avoid build dependencies
    const userSchema = new mongoose.Schema({
      username: { type: String, required: true, unique: true },
      email: { type: String, required: true, unique: true },
      password: { type: String, required: true },
      firstName: { type: String, required: true },
      lastName: { type: String, required: true },
      role: { type: String, required: true },
      phone: { type: String, required: true },
      active: { type: Boolean, default: true }
    }, { timestamps: true });
    
    const User = mongoose.model('User', userSchema);
    
    // Check if admin already exists
    const existingAdmin = await User.findOne({ username: 'admin' });
    if (existingAdmin) {
      console.log('Admin user already exists!');
      console.log('Username: admin');
      console.log('To reset password, delete the user and run this script again.');
      process.exit(0);
    }
    
    // Hash password
    console.log('Creating admin user...');
    const hashedPassword = await bcrypt.hash('admin123', 10);
    
    // Create admin user
    await User.create({
      username: 'admin',
      email: 'admin@billgenie.com',
      password: hashedPassword,
      firstName: 'System',
      lastName: 'Administrator',
      role: 'admin',
      phone: '+1234567890',
      active: true
    });
    
    console.log('\n✅ Admin user created successfully!');
    console.log('\nLogin credentials:');
    console.log('  Username: admin');
    console.log('  Password: admin123');
    console.log('\n⚠️  Please change this password after first login!\n');
    
    process.exit(0);
  } catch (error) {
    console.error('Error creating admin user:', error);
    process.exit(1);
  }
}

createAdmin();
