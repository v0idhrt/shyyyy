-- users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    fio TEXT,
    email TEXT,
    phone TEXT,
    birth_date TEXT,
    address TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- seed admin user
INSERT OR IGNORE INTO users (id, login, password, fio, email, phone, birth_date, address)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'admin',
    'admin',
    'Admin User',
    'admin@example.com',
    '+10000000000',
    '1970-01-01',
    'Admin Street 1'
);
