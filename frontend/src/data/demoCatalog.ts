import { Category, StoreProduct } from '../api/types';

export const DEMO_CATEGORIES: Category[] = [
  {
    id: 'cat-1',
    name: 'Dairy & Breakfast',
    slug: 'dairy-breakfast',
    description: 'Fresh milk, curd, butter, paneer, and farm eggs',
  },
  {
    id: 'cat-2',
    name: 'Fresh Fruits & Veggies',
    slug: 'fruits-vegetables',
    description: 'Farm fresh produce delivered in minutes',
  },
  {
    id: 'cat-3',
    name: 'Snacks & Munchies',
    slug: 'snacks-munchies',
    description: 'Crisps, chips, nuts, and evening bites',
  },
  {
    id: 'cat-4',
    name: 'Beverages & Juices',
    slug: 'beverages',
    description: 'Cold sodas, real fruit juices, and iced teas',
  },
  {
    id: 'cat-5',
    name: 'Instant Food & Noodles',
    slug: 'instant-food',
    description: '2-minute noodles, soups, and ready-to-eat meals',
  },
  {
    id: 'cat-6',
    name: 'Bakery & Bread',
    slug: 'bakery-bread',
    description: 'Artisan brown bread, buns, and cookies',
  },
];

export const DEMO_PRODUCTS: StoreProduct[] = [
  {
    product_id: 'prod-001',
    name: 'Amul Taaza Homogenised Toned Milk',
    sku: 'MILK-AMUL-1L',
    price_paise: 7400, // ₹74.00
    available_quantity: 45,
    unit: '1 L',
    category_id: 'cat-1',
  },
  {
    product_id: 'prod-002',
    name: 'Mother Dairy Classic Dahi Pouch',
    sku: 'DAHI-MD-400G',
    price_paise: 3500, // ₹35.00
    available_quantity: 30,
    unit: '400 g',
    category_id: 'cat-1',
  },
  {
    product_id: 'prod-003',
    name: 'Farm Fresh Organic Brown Eggs',
    sku: 'EGGS-FARM-6PK',
    price_paise: 6800, // ₹68.00
    available_quantity: 22,
    unit: '6 pcs',
    category_id: 'cat-1',
  },
  {
    product_id: 'prod-004',
    name: 'Fresh Robusta Cavendish Bananas',
    sku: 'FRUIT-BANANA-500G',
    price_paise: 4800, // ₹48.00
    available_quantity: 40,
    unit: '500 g',
    category_id: 'cat-2',
  },
  {
    product_id: 'prod-005',
    name: 'Farm Fresh Hybrid Tomatoes',
    sku: 'VEG-TOMATO-500G',
    price_paise: 2900, // ₹29.00
    available_quantity: 35,
    unit: '500 g',
    category_id: 'cat-2',
  },
  {
    product_id: 'prod-006',
    name: 'Britannia 100% Whole Wheat Bread',
    sku: 'BREAD-WHT-400G',
    price_paise: 5000, // ₹50.00
    available_quantity: 18,
    unit: '400 g',
    category_id: 'cat-6',
  },
  {
    product_id: 'prod-007',
    name: "Lay's India's Magic Masala Potato Chips",
    sku: 'CHIPS-LAYS-MASALA',
    price_paise: 2000, // ₹20.00
    available_quantity: 60,
    unit: '50 g',
    category_id: 'cat-3',
  },
  {
    product_id: 'prod-008',
    name: 'Maggi 2-Minute Masala Instant Noodles 4-Pack',
    sku: 'NOODLE-MAGGI-4PK',
    price_paise: 5600, // ₹56.00
    available_quantity: 50,
    unit: '280 g',
    category_id: 'cat-5',
  },
  {
    product_id: 'prod-009',
    name: 'Coca-Cola Zero Sugar Soft Drink Can',
    sku: 'BEV-COKE-ZERO',
    price_paise: 4000, // ₹40.00
    available_quantity: 25,
    unit: '300 ml',
    category_id: 'cat-4',
  },
  {
    product_id: 'prod-010',
    name: 'Tropicana 100% Real Orange Juice',
    sku: 'BEV-TROP-ORANGE',
    price_paise: 13000, // ₹130.00
    available_quantity: 15,
    unit: '1 L',
    category_id: 'cat-4',
  },
  {
    product_id: 'prod-011',
    name: 'Epigamia Greek Yogurt Natural Strawberry',
    sku: 'YOGURT-EPIG-STR',
    price_paise: 6000, // ₹60.00
    available_quantity: 12,
    unit: '90 g',
    category_id: 'cat-1',
  },
  {
    product_id: 'prod-012',
    name: 'Quaker Whole Rolled Oats Pouch',
    sku: 'OATS-QUAKER-1KG',
    price_paise: 19900, // ₹199.00
    available_quantity: 8,
    unit: '1 kg',
    category_id: 'cat-1',
  },
];
