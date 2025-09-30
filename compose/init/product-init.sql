CREATE TABLE IF NOT EXISTS product (
  ID INT AUTO_INCREMENT PRIMARY KEY,
  NAME VARCHAR(255),
  DESCRIPTION VARCHAR(255),
  PRICE DOUBLE,
  IMAGE_URL VARCHAR(255)
);

INSERT INTO product (NAME, DESCRIPTION, PRICE, IMAGE_URL)
VALUES
  ('Red Shirt',      'Comfortable cotton shirt',      19.99, '/images/red-shirt.jpg'),
  ('Blue Jeans',     'Slim fit denim pants',          49.99, '/images/blue-jeans.jpg'),
  ('Sneakers',       'Running shoes',                 69.99, '/images/sneakers.jpg'),
  ('Black Hat',      'Stylish black baseball cap',    14.99, '/images/black-hat.jpg'),
  ('White Socks',    'Soft cotton ankle socks (5-pair)', 5.99, '/images/white-socks.jpg'),
  ('Leather Belt',   'Genuine leather belt, brown',   24.99, '/images/leather-belt.jpg'),
  ('Sports Jacket',  'Lightweight windbreaker jacket', 59.99, '/images/sports-jacket.jpg'),
  ('Sunglasses',     'UV-protection aviator style',   29.99, '/images/sunglasses.jpg'),
  ('Wristwatch',     'Water-resistant analog watch',   89.99, '/images/wristwatch.jpg'),
  ('Backpack',       'Durable 20L travel backpack',    39.99, '/images/backpack.jpg');