-- Products master
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Batch/session untuk pengukuran produk (dipakai juga untuk report header fields)
CREATE TABLE IF NOT EXISTS product_measurement_batches (
    id SERIAL PRIMARY KEY,
    product_id INT NULL REFERENCES products(id) ON DELETE SET NULL,
    no_machine VARCHAR(100) NOT NULL DEFAULT '',
    batch_number VARCHAR(100) NOT NULL DEFAULT '',
    start_date TIMESTAMP NOT NULL,
    finish_date TIMESTAMP NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'in_progress', -- in_progress, submitted, archived
    created_by INT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_product_measurement_batches_product_id ON product_measurement_batches(product_id);
CREATE INDEX IF NOT EXISTS idx_product_measurement_batches_status ON product_measurement_batches(status);
CREATE INDEX IF NOT EXISTS idx_product_measurement_batches_created_at ON product_measurement_batches(created_at);

-- Detail measurements per item (baris-baris seperti Master Sheet: MEASUREMENT ITEM / TYPE / SAMPLE INDEX / RESULT)
CREATE TABLE IF NOT EXISTS product_measurements (
    id SERIAL PRIMARY KEY,
    batch_id INT NOT NULL REFERENCES product_measurement_batches(id) ON DELETE CASCADE,
    measurement_item VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'Single', -- Single / Non / etc
    sample_index INT NULL,
    result VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_product_measurements_batch_id ON product_measurements(batch_id);
CREATE INDEX IF NOT EXISTS idx_product_measurements_measurement_item ON product_measurements(measurement_item);

