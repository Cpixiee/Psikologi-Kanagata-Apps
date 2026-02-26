-- Create device_verifications table for device security validation
CREATE TABLE device_verifications (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL,
    token VARCHAR(255) UNIQUE NOT NULL,
    is_verified BOOLEAN DEFAULT FALSE,
    is_rejected BOOLEAN DEFAULT FALSE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_device_verifications_user_id ON device_verifications(user_id);
CREATE INDEX idx_device_verifications_device_id ON device_verifications(device_id);
CREATE INDEX idx_device_verifications_token ON device_verifications(token);
