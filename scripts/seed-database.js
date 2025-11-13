require('dotenv').config();
const mongoose = require('mongoose');

async function seedDatabase() {
  try {
    const mongoURI = process.env.MONGODB_URI || 'mongodb://localhost:27017/billgenie';
    console.log('Connecting to MongoDB...');
    await mongoose.connect(mongoURI);
    console.log('Connected to MongoDB');
    
    // Define schemas
    const menuItemSchema = new mongoose.Schema({
      name: String,
      description: String,
      category: String,
      price: Number,
      available: Boolean,
      preparationTime: Number,
      ingredients: [String]
    }, { timestamps: true });
    
    const tableSchema = new mongoose.Schema({
      tableNumber: Number,
      capacity: Number,
      status: String,
      location: String
    }, { timestamps: true });
    
    const MenuItem = mongoose.model('MenuItem', menuItemSchema);
    const Table = mongoose.model('Table', tableSchema);
    
    // Seed menu items
    console.log('Creating sample menu items...');
    const menuItems = await MenuItem.insertMany([
      {
        name: 'Margherita Pizza',
        description: 'Classic pizza with tomato sauce, mozzarella, and basil',
        category: 'Pizza',
        price: 12.99,
        available: true,
        preparationTime: 15,
        ingredients: ['dough', 'tomato sauce', 'mozzarella', 'basil']
      },
      {
        name: 'Caesar Salad',
        description: 'Fresh romaine lettuce with Caesar dressing and croutons',
        category: 'Salads',
        price: 8.99,
        available: true,
        preparationTime: 10,
        ingredients: ['romaine lettuce', 'Caesar dressing', 'croutons', 'parmesan']
      },
      {
        name: 'Cheeseburger',
        description: 'Juicy beef patty with cheese, lettuce, and tomato',
        category: 'Burgers',
        price: 10.99,
        available: true,
        preparationTime: 12,
        ingredients: ['beef patty', 'cheese', 'bun', 'lettuce', 'tomato']
      },
      {
        name: 'Pasta Carbonara',
        description: 'Creamy pasta with bacon and parmesan',
        category: 'Pasta',
        price: 14.99,
        available: true,
        preparationTime: 18,
        ingredients: ['pasta', 'bacon', 'cream', 'parmesan', 'eggs']
      },
      {
        name: 'Chocolate Cake',
        description: 'Rich chocolate cake with chocolate frosting',
        category: 'Desserts',
        price: 6.99,
        available: true,
        preparationTime: 5,
        ingredients: ['chocolate cake', 'chocolate frosting']
      }
    ]);
    console.log(`✅ Created ${menuItems.length} menu items`);
    
    // Seed tables
    console.log('Creating sample tables...');
    const tables = await Table.insertMany([
      { tableNumber: 1, capacity: 2, status: 'available', location: 'Main Hall' },
      { tableNumber: 2, capacity: 4, status: 'available', location: 'Main Hall' },
      { tableNumber: 3, capacity: 4, status: 'available', location: 'Main Hall' },
      { tableNumber: 4, capacity: 6, status: 'available', location: 'Main Hall' },
      { tableNumber: 5, capacity: 2, status: 'available', location: 'Window Side' },
      { tableNumber: 6, capacity: 2, status: 'available', location: 'Window Side' },
      { tableNumber: 7, capacity: 8, status: 'available', location: 'Private Room' },
      { tableNumber: 8, capacity: 4, status: 'available', location: 'Outdoor Patio' }
    ]);
    console.log(`✅ Created ${tables.length} tables`);
    
    console.log('\n✅ Database seeded successfully!');
    console.log('\nSummary:');
    console.log(`  - ${menuItems.length} menu items created`);
    console.log(`  - ${tables.length} tables created`);
    console.log('\nYou can now start using the application!\n');
    
    process.exit(0);
  } catch (error) {
    console.error('Error seeding database:', error);
    process.exit(1);
  }
}

seedDatabase();
