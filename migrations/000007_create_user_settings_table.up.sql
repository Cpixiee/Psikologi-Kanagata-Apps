-- Create user_settings table
CREATE TABLE user_settings (
    id SERIAL PRIMARY KEY,
    user_id INTEGER UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- New for you notifications
    notif_new_for_you_email BOOLEAN DEFAULT TRUE,
    notif_new_for_you_browser BOOLEAN DEFAULT TRUE,
    notif_new_for_you_app BOOLEAN DEFAULT TRUE,
    -- Account activity notifications
    notif_activity_email BOOLEAN DEFAULT TRUE,
    notif_activity_browser BOOLEAN DEFAULT TRUE,
    notif_activity_app BOOLEAN DEFAULT TRUE,
    -- Browser login notifications
    notif_browser_login_email BOOLEAN DEFAULT TRUE,
    notif_browser_login_browser BOOLEAN DEFAULT TRUE,
    notif_browser_login_app BOOLEAN DEFAULT FALSE,
    -- Device link notifications
    notif_device_link_email BOOLEAN DEFAULT TRUE,
    notif_device_link_browser BOOLEAN DEFAULT FALSE,
    notif_device_link_app BOOLEAN DEFAULT FALSE,
    notification_timing VARCHAR(50) DEFAULT 'online',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
