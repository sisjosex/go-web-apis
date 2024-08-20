CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    first_name VARCHAR(255) NOT NULL DEFAULT '',
    last_name VARCHAR(255) NOT NULL DEFAULT '',
    phone VARCHAR(30) NOT NULL DEFAULT '',
    birthday DATE,

    /* Credentials */
    email VARCHAR(255) NOT NULL UNIQUE DEFAULT '',
    password VARCHAR(255) NOT NULL DEFAULT '',

    /* Social login */
    facebook_id VARCHAR(255), -- Facebook
    google_id VARCHAR(255),   -- Google
    hotmail_id VARCHAR(255)   -- Hotmail
);