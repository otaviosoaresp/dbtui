CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(200),
    status VARCHAR(20) DEFAULT 'active'
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    category_id INTEGER
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES customers(id),
    total NUMERIC(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE order_items (
    order_id INTEGER NOT NULL REFERENCES orders(id),
    product_id INTEGER NOT NULL REFERENCES products(id),
    quantity INTEGER NOT NULL DEFAULT 1,
    price NUMERIC(10, 2) NOT NULL,
    PRIMARY KEY (order_id, product_id)
);

CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    parent_id INTEGER REFERENCES categories(id)
);

ALTER TABLE products ADD CONSTRAINT fk_products_category
    FOREIGN KEY (category_id) REFERENCES categories(id);

CREATE TABLE tags (
    id SERIAL PRIMARY KEY,
    label VARCHAR(50) NOT NULL
);

CREATE TABLE product_tags (
    product_id INTEGER NOT NULL REFERENCES products(id),
    tag_id INTEGER NOT NULL REFERENCES tags(id),
    PRIMARY KEY (product_id, tag_id)
);

-- table without PK
CREATE TABLE audit_log (
    event_type VARCHAR(50),
    table_name VARCHAR(100),
    record_id INTEGER,
    changed_at TIMESTAMP DEFAULT NOW(),
    changed_by VARCHAR(100)
);

-- employees with self-referential FK
CREATE TABLE employees (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    manager_id INTEGER REFERENCES employees(id),
    department VARCHAR(50)
);

CREATE VIEW active_customers AS
    SELECT id, name, email FROM customers WHERE status = 'active';

CREATE MATERIALIZED VIEW order_summary AS
    SELECT
        c.name AS customer_name,
        COUNT(o.id) AS order_count,
        SUM(o.total) AS total_spent
    FROM customers c
    LEFT JOIN orders o ON o.customer_id = c.id
    GROUP BY c.name;

INSERT INTO categories (name, parent_id) VALUES
    ('Electronics', NULL),
    ('Laptops', 1),
    ('Phones', 1),
    ('Clothing', NULL);

INSERT INTO customers (name, email, status) VALUES
    ('Alice Johnson', 'alice@example.com', 'active'),
    ('Bob Smith', 'bob@example.com', 'active'),
    ('Carol White', 'carol@example.com', 'inactive');

INSERT INTO products (name, price, category_id) VALUES
    ('MacBook Pro', 2499.99, 2),
    ('iPhone 16', 999.99, 3),
    ('T-Shirt', 29.99, 4);

INSERT INTO tags (label) VALUES ('bestseller'), ('new'), ('sale');

INSERT INTO product_tags (product_id, tag_id) VALUES (1, 1), (2, 1), (2, 2), (3, 3);

INSERT INTO orders (customer_id, total, status) VALUES
    (1, 2499.99, 'completed'),
    (1, 999.99, 'completed'),
    (2, 29.99, 'pending'),
    (3, 3499.98, 'shipped');

INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
    (1, 1, 1, 2499.99),
    (2, 2, 1, 999.99),
    (3, 3, 1, 29.99),
    (4, 1, 1, 2499.99),
    (4, 2, 1, 999.99);

INSERT INTO employees (name, manager_id, department) VALUES
    ('CEO', NULL, 'Executive'),
    ('CTO', 1, 'Engineering'),
    ('VP Engineering', 2, 'Engineering'),
    ('Senior Dev', 3, 'Engineering'),
    ('Junior Dev', 4, 'Engineering');

REFRESH MATERIALIZED VIEW order_summary;
